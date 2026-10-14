package server

import (
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
	"github.com/vriesdemichael/mm-mcp/internal/fileview"
	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// A file attached to a post is also an MCP resource (ADR-029): it has a fixed
// content and a type, and as a resource it has an address a client can hold
// on to and read when it needs it, rather than when a tool answers. The
// address is mattermost://<server>/files/<id>, not the file's https address,
// which a client would try to fetch itself, without the credential. Reading
// it converts the file as read_file does. read_file and save_file stay: in
// many clients only the person can open a resource, and a resource writes
// nothing to disk.
//
// A resource goes to the client, which shows it, saves it or puts it in the
// conversation, rather than straight into the model's context, so read_file's
// limits do not bound it. MCP sends a resource whole, in one message, and a
// client refuses a message past its own limit by dropping the request or the
// whole session, so the message is capped, MM_MCP_RESOURCE_MAX_MB, 10 MiB by
// default, and refused here first with a way to read the file instead. The
// file downloaded is bounded by Mattermost's own limit on a file, which an
// administrator sets for every upload: a file past it, most likely attached
// before the limit was lowered, is refused before it is fetched.

// ResourceSpec is a resource template mm-mcp offers, with the operations
// reading a resource and completing the template's argument call, accounted
// for as a tool's are (ADR-028).
type ResourceSpec struct {
	// Name is the template's name.
	Name string
	// Template is the template a server configured so lists.
	Template func(cfg config.Config) *mcp.ResourceTemplate
	// Read reads one resource of the template.
	Read func(clientFor ClientFor, cfg config.Config) mcp.ResourceHandler
	// Uses is every operation reading a resource calls.
	Uses []Use
	// Complete suggests values of the template's argument from what the
	// person typed.
	Complete func(ctx context.Context, client *mattermost.Client, argument, typed string) (mcp.CompletionResultDetails, error)
	// CompletionUses is every operation completing an argument calls.
	CompletionUses []Use
}

// AllResources is every resource template mm-mcp offers, whatever the
// configuration: each only reads.
func AllResources() []ResourceSpec {
	return []ResourceSpec{fileResourceSpec(), previewResourceSpec(), pictureResourceSpec(), emojiResourceSpec()}
}

// fileResource is the address of a file of the server at server as a
// resource.
func fileResource(server *url.URL, id string) string {
	return resourceBase(server) + "/files/" + id
}

// fileResourceID is the file a resource address names on the server at
// server.
func fileResourceID(server, address *url.URL) (string, bool) {
	id, ok := resourcePart(server, address, "/files/", "")
	return id, ok && mattermostID.MatchString(id)
}

// fileURI is a file's resource address on the client's server.
func fileURI(client *mattermost.Client, id string) string {
	server, err := url.Parse(client.Address())
	if err != nil || id == "" {
		return ""
	}
	return fileResource(server, id)
}

// fileArgument is the file template's one argument, named as tools name it.
const fileArgument = "file_id"

func fileResourceSpec() ResourceSpec {
	return ResourceSpec{
		Name: "file",
		Template: func(cfg config.Config) *mcp.ResourceTemplate {
			return &mcp.ResourceTemplate{
				Name:        "file",
				Title:       "Mattermost file",
				URITemplate: fileResource(serverAddress(cfg), "{"+fileArgument+"}"),
				Description: "A file attached to a post, converted as read_file converts it: text, the text of a Word, PowerPoint or Excel " +
					"file, and an archive's listing as text; an image as an image, scaled as read_file scales it; a PDF, audio, video " +
					"or any other file as itself. A resource is read whole" + resourceBounds(cfg) + "; read_file reads a long text in windows.",
			}
		},
		Read: readFileResource,
		Uses: append(fileOperationUses(), Use{Operation: "GetClientConfig", Params: map[string]Coverage{}}),
		Complete: func(ctx context.Context, client *mattermost.Client, argument, typed string) (mcp.CompletionResultDetails, error) {
			none := mcp.CompletionResultDetails{Values: []string{}}
			typed = strings.TrimSpace(typed)
			if argument != fileArgument || typed == "" || isLink(typed) || mattermostID.MatchString(typed) {
				return none, nil
			}
			found, err := client.SearchFiles(ctx, mattermost.FileSearch{Terms: typed + "*", PerPage: maxCompletions})
			if err != nil {
				return none, err
			}
			var files []*model.FileInfo
			for _, id := range found.Order {
				if file := found.FileInfos[id]; file != nil {
					files = append(files, file)
				}
			}
			slices.SortStableFunc(files, func(a, b *model.FileInfo) int {
				if a.CreateAt != b.CreateAt {
					return int(b.CreateAt - a.CreateAt)
				}
				return strings.Compare(a.Id, b.Id)
			})
			values := make([]string, 0, len(files))
			for _, file := range files {
				values = append(values, file.Id)
			}
			// Mattermost says nothing of how many more match; a full page may
			// have more behind it.
			return mcp.CompletionResultDetails{Values: values, HasMore: len(values) >= maxCompletions}, nil
		},
		CompletionUses: []Use{{
			Operation: "SearchFiles",
			Params: map[string]Coverage{
				"team_id":                       Omitted("a file is suggested from every team the person can read"),
				"body.terms":                    SetBy(fileArgument),
				"body.is_or_search":             Omitted("each word typed narrows the files, as in Mattermost's search box"),
				"body.page":                     Fixed("0", "a suggestion is the first, newest files that match"),
				"body.per_page":                 Fixed(fmt.Sprint(maxCompletions), "the most values one completion holds"),
				"body.time_zone_offset":         Omitted("nothing typed is a day"),
				"body.include_deleted_channels": Omitted("archived channels are left out of a search, as they are out of search_files"),
			},
		}},
	}
}

// maxCompletions is the most values MCP lets one completion hold.
const maxCompletions = 100

// resourceBounds says what bounds a resource on a server configured so.
func resourceBounds(cfg config.Config) string {
	if most := cfg.ResourceBytes(); most > 0 {
		return ", in one message of up to " + sizeOf(most)
	}
	return ""
}

// serverAddress is the configured server's address.
func serverAddress(cfg config.Config) *url.URL {
	server, err := url.Parse(strings.TrimRight(cfg.URL, "/"))
	if err != nil {
		panic(fmt.Sprintf("the server's address %q: %v", cfg.URL, err))
	}
	return server
}

// readFileResource reads a file's resource: the file, converted as read_file
// converts it, whole.
func readFileResource(clientFor ClientFor, cfg config.Config) mcp.ResourceHandler {
	server := serverAddress(cfg)
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := request.Params.URI
		given, err := url.Parse(uri)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		id, ok := fileResourceID(server, given)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		client, err := clientFor(ctx, request)
		if err != nil {
			return nil, err
		}
		info, err := client.FileInfo(ctx, id)
		if notFound(err) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if err != nil {
			return nil, err
		}
		contents, err := wholeFile(ctx, client, info, cfg, uri)
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
	}
}

// plainText is the type of text extracted from a file, or listed from it.
const plainText = "text/plain; charset=utf-8"

// wholeFile is a file as a resource: converted as read_file converts it, and
// returned whole, as a resource is read. A file larger than Mattermost now
// takes is refused before it is fetched, and one whose message would be
// larger than the server's cap on a resource is refused after; a text the
// conversion cut short always is. Each refusal says what reads the file
// instead. GetClientConfig, GetFile.
func wholeFile(ctx context.Context, client *mattermost.Client, info *model.FileInfo, cfg config.Config, uri string) (*mcp.ResourceContents, error) {
	opens := client.Permalink(info.PostId)
	instead := "read_file reads a text in windows, and save_file saves a file, and the person can open it from its post: " + opens
	clientConfig, err := client.ClientConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := withinUploadLimit(info, maxFileSize(clientConfig), instead); err != nil {
		return nil, err
	}
	data, err := client.File(ctx, info.Id)
	if err != nil {
		return nil, err
	}
	view, err := fileview.Read(ctx, fileview.Request{Name: info.Name, WebURL: opens}, data)
	if err != nil {
		return nil, err
	}
	var contents *mcp.ResourceContents
	isPDF := view.MIMEType == pdfType || info.MimeType == pdfType
	switch {
	case isPDF:
		// A PDF is the client's to convert, as read_file leaves it.
		contents = &mcp.ResourceContents{MIMEType: pdfType, Blob: data}
	case view.Window != nil:
		if view.Partial {
			return nil, fmt.Errorf("the text of %s stops short of what the file holds, as read_file's header for it says, and a resource is the whole of it; %s",
				info.Name, instead)
		}
		mimeType := plainText
		if view.Kind == fileview.KindText && view.MIMEType != "" {
			mimeType = view.MIMEType
		}
		contents = &mcp.ResourceContents{MIMEType: mimeType, Text: view.Whole}
		if view.Whole == "" {
			// Text resource contents must hold text, and an empty one would be
			// left out of the answer altogether.
			contents = &mcp.ResourceContents{MIMEType: mimeType, Blob: []byte{}}
		}
	case view.Image != nil:
		contents = &mcp.ResourceContents{MIMEType: view.Image.MIMEType, Blob: view.Image.Data}
	default:
		// Audio, video and any other file as its bytes, which read_file
		// returns only when small.
		contents = &mcp.ResourceContents{MIMEType: cmp.Or(view.MIMEType, info.MimeType, "application/octet-stream"), Blob: data}
	}
	contents.URI = uri
	if most := cfg.ResourceBytes(); most > 0 {
		if sent := messageSize(contents); sent > most {
			return nil, fmt.Errorf("%s is %s as one message, more than the %s a resource is sent in (%s); %s",
				info.Name, sizeOf(sent), sizeOf(most), config.EnvResourceMaxMB, instead)
		}
	}
	return contents, nil
}

// maxFileSize is the largest file Mattermost takes, from the configuration it
// tells its clients, or 0 when it does not say.
func maxFileSize(clientConfig map[string]string) int64 {
	if size, err := strconv.ParseInt(clientConfig["MaxFileSize"], 10, 64); err == nil && size > 0 {
		return size
	}
	return 0
}

// withinUploadLimit refuses a file larger than the most Mattermost takes in
// one, limit, where it says: a resource read downloads no more than an upload
// could hold. Every file uploaded was within the limit then, so a larger one
// most likely came before an administrator lowered it.
func withinUploadLimit(info *model.FileInfo, limit int64, instead string) error {
	if limit <= 0 || info.Size <= limit {
		return nil
	}
	return fmt.Errorf("%s is %s, more than the %s Mattermost now takes in one file, its Maximum File Size, which its "+
		"administrator sets; a resource reads no file larger than Mattermost takes; %s",
		info.Name, sizeOf(info.Size), sizeOf(limit), instead)
}

// messageEnvelope is more than the rest of the message a resource's contents
// are sent in takes: the JSON-RPC response around them, its id, and the
// contents' own field names.
const messageEnvelope = 256

// messageSize is about how large the message that answers a read with
// contents is, never less: what a client's limit on one message measures.
func messageSize(contents *mcp.ResourceContents) int64 {
	return sentSize(contents) + int64(len(contents.URI)+len(contents.MIMEType)) + messageEnvelope
}

// sentSize is about how large contents are as sent, never less: a blob as its
// base64, and text with what JSON escapes in it, counting every escape as
// Go's encoder writes the longest, \u0000.
func sentSize(contents *mcp.ResourceContents) int64 {
	if contents.Text == "" {
		return int64(base64.StdEncoding.EncodedLen(len(contents.Blob)))
	}
	size := int64(0)
	for _, r := range contents.Text {
		switch {
		case r == utf8.RuneError:
			size += 6
		case r < 0x20, r == '<', r == '>', r == '&', r == '\u2028', r == '\u2029':
			size += 6
		case r == '"', r == '\\':
			size += 2
		default:
			size += int64(utf8.RuneLen(r))
		}
	}
	return size
}

// sizeOf is a size in bytes as a person reads it.
func sizeOf(size int64) string {
	switch {
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(size)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", size)
}

// completion is the server's answer to a request to complete an argument:
// the template the request names suggests values; any other reference has
// none.
func completion(clientFor ClientFor, cfg config.Config, resources []ResourceSpec) func(context.Context, *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, request *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		none := &mcp.CompleteResult{Completion: mcp.CompletionResultDetails{Values: []string{}}}
		ref := request.Params.Ref
		if ref == nil || ref.Type != "ref/resource" {
			return none, nil
		}
		for _, resource := range resources {
			if resource.Complete == nil || resource.Template(cfg).URITemplate != ref.URI {
				continue
			}
			client, err := clientFor(ctx, request)
			if err != nil {
				return nil, err
			}
			details, err := resource.Complete(ctx, client, request.Params.Argument.Name, request.Params.Argument.Value)
			if err != nil {
				return nil, err
			}
			return &mcp.CompleteResult{Completion: details}, nil
		}
		return none, nil
	}
}

// registerResources adds every resource template to a server.
func registerResources(server *mcp.Server, clientFor ClientFor, cfg config.Config, resources []ResourceSpec) {
	for _, resource := range resources {
		server.AddResourceTemplate(resource.Template(cfg), resource.Read(clientFor, cfg))
	}
}

// fileLinks are resource links to the files an answer names, each once, in
// the order the answer names them, so a client can read one when it needs it.
func fileLinks(answer reflect.Value) []mcp.Content {
	var links []mcp.Content
	seen := map[string]bool{}
	var walk func(value reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Pointer:
			if !value.IsNil() {
				walk(value.Elem())
			}
		case reflect.Slice, reflect.Array:
			for i := range value.Len() {
				walk(value.Index(i))
			}
		case reflect.Struct:
			if value.Type() == reflect.TypeFor[Attachment]() {
				if file, ok := value.Interface().(Attachment); ok && file.URI != "" && !seen[file.URI] {
					seen[file.URI] = true
					links = append(links, fileLink(file))
				}
				return
			}
			for i := range value.NumField() {
				if field := value.Type().Field(i); field.IsExported() || field.Anonymous {
					walk(value.Field(i))
				}
			}
		}
	}
	walk(answer)
	return links
}

// fileLink is a resource link to an attached file.
func fileLink(file Attachment) *mcp.ResourceLink {
	link := &mcp.ResourceLink{URI: file.URI, Name: cmp.Or(file.Name, file.ID), MIMEType: file.MIMEType}
	if file.Size > 0 {
		size := file.Size
		link.Size = &size
	}
	return link
}
