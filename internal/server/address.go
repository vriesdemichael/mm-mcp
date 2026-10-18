package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A write others see names where it goes by its address (ADR-030):
// mattermost:// and the path the place's link has in Mattermost's web app.
//
//	mattermost://<server>/<team>                          a team
//	mattermost://<server>/<team>/channels/<channel>       a channel
//	mattermost://<server>/<team>/messages/@<username>     a direct message
//	mattermost://<server>/<team>/messages/<name>          a group message
//	<a channel's address>/<post id>                       a post in it
//
// An address is made of names, so whoever approves the call, a person at the
// client's prompt or its AI reviewer, sees where it goes. It is matched
// exactly, so the model must know the place or have looked it up: an id, a
// name or a link is refused, with where to read the address. Every answer
// that describes a team, a channel or a post carries its address, and the
// tools that read take one wherever they take an id.

// asAddress is the address of the place a link opens: the link under the
// mattermost scheme. Empty for no link.
func asAddress(link string) string {
	if link == "" {
		return ""
	}
	address, err := url.Parse(link)
	if err != nil || address.Host == "" {
		return ""
	}
	address.Scheme, address.Host = fileResourceScheme, strings.ToLower(address.Host)
	return address.String()
}

// postAddress is a post's address: its channel's, then its id.
func postAddress(channelAddress, postID string) string {
	if channelAddress == "" || postID == "" {
		return ""
	}
	return channelAddress + "/" + postID
}

// addressArgs are the id arguments a write others see takes as an address,
// with what each refusal tells the model to give and where to read it.
var addressArgs = map[string]struct{ shape, where string }{
	channelArg: {"<team>/channels/<channel>, or <team>/messages/@<username> for a direct message", "get_user_channels, get_channel_info and every post give each channel's address"},
	postArg:    {"<team>/channels/<channel>/<post id>: its channel's address, then its id", "read_post, read_channel and search_posts give each post's address"},
	rootArg:    {"<team>/channels/<channel>/<post id>: its channel's address, then its id", "read_post, read_channel and search_posts give each post's address"},
	teamArg:    {"<team>", "get_user_teams gives each team's address"},
}

// addressNote is what an addressed argument's description says it takes.
func addressNote(arg string) string {
	return fmt.Sprintf("its address, mattermost://<server>/%s; %s. An id, a name or a link is refused: the address names the place exactly, and shows it to whoever approves the call",
		addressArgs[arg].shape, addressArgs[arg].where)
}

// addressed is a write others see, whose channel, post and team arguments
// take an address alone. Their descriptions say so, and the operations reading
// an address may call replace those reading a name.
func addressed(spec Spec) Spec {
	schema, ok := spec.Tool.InputSchema.(*jsonschema.Schema)
	if !ok {
		return spec
	}
	fields := map[string]bool{}
	for name := range schema.Properties {
		if _, ok := addressArgs[name]; ok {
			fields[name] = true
		}
	}
	if len(fields) == 0 {
		return spec
	}
	spec.Addressed = true
	for name, property := range schema.Properties {
		if fields[name] {
			base := strings.TrimSuffix(strings.TrimSuffix(property.Description, " ("+idNotes[name]+")"), idNotes[name])
			if base == "" {
				property.Description = addressNote(name)
			} else {
				property.Description = base + " (" + addressNote(name) + ")"
			}
		}
	}
	var kept []Use
	for _, use := range spec.Uses {
		// A name is never searched for: an address names its channel exactly.
		if use.Operation != "SearchChannels" {
			kept = append(kept, use)
		}
	}
	extra := []Use{}
	channel, post, root := fields[channelArg], fields[postArg], fields[rootArg]
	if channel || post || root {
		arg := channelArg
		if !channel {
			arg = postArg
			if root {
				arg = rootArg
			}
		}
		for _, use := range channelLookupUses(arg, Omitted("an address names its team")) {
			if use.Operation != "SearchChannels" && use.Operation != "GetChannel" {
				extra = append(extra, use)
			}
		}
	}
	spec.Uses = uses(kept, extra)
	return spec
}

// resolveAddress is the id an addressed argument's address means, refusing
// anything but an address of the place the argument names.
func resolveAddress(ctx context.Context, request *mcp.CallToolRequest, clientFor ClientFor, arg, given string) (string, error) {
	client, err := clientFor(ctx, request)
	if err != nil {
		return "", err
	}
	server, err := url.Parse(client.Address())
	if err != nil {
		return "", err
	}
	wanted := addressArgs[arg]
	refuse := func(what string) error {
		return fmt.Errorf("%s is %s, but this takes an address, %s/%s, which names the place exactly and shows it to whoever approves the call; %s",
			strings.TrimSpace(given), what, resourceBase(server), wanted.shape, wanted.where)
	}
	l, isLink, err := parseLink(client, given)
	var foreign foreignLinkError
	if errors.As(err, &foreign) {
		return "", refuse("a link to " + foreign.host)
	}
	if err != nil {
		return "", err
	}
	switch {
	case !isLink && mattermostID.MatchString(bareName(given)):
		return "", refuse("an id")
	case !isLink:
		return "", refuse("a name")
	case !l.address:
		return "", refuse("a link")
	}
	switch arg {
	case teamArg:
		if l.kind != linkTeam {
			return "", refuse(string(l.kind) + " address")
		}
		return ownTeam(ctx, client, given)
	case channelArg:
		if l.post != "" || (l.kind != linkChannel && l.kind != linkMessages) {
			return "", refuse(string(l.kind) + " address")
		}
		if err := byName(l); err != nil {
			return "", err
		}
		channel, err := findChannel(ctx, client, given, "")
		return channel.ID, err
	}
	// A post's address: its channel's, then its id.
	if l.post == "" || (l.kind != linkChannel && l.kind != linkMessages) {
		return "", refuse(string(l.kind) + " address")
	}
	if err := byName(l); err != nil {
		return "", err
	}
	channel, err := findChannel(ctx, client, strings.TrimSuffix(l.given, "/"+l.post), "")
	if err != nil {
		return "", err
	}
	post, err := client.Post(ctx, l.post)
	if err != nil {
		return "", err
	}
	if post.ChannelId != channel.ID {
		return "", fmt.Errorf("the post %s is not in %s; read_post gives its address", l.post, strings.TrimSuffix(l.given, "/"+l.post))
	}
	return post.Id, nil
}

// byName refuses an address that names its channel by id: an address is made
// of names, which say where it goes.
func byName(l link) error {
	if mattermostID.MatchString(l.name) || (l.kind == linkChannel && (directName.MatchString(l.name) || groupName.MatchString(l.name))) {
		return fmt.Errorf("%s names its channel by id; its address names it by name, as get_channel_info gives it", l.given)
	}
	return nil
}
