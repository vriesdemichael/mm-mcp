//go:build live

package live

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/server"
)

// A write others see names its places by address alone: mattermost:// and the
// path of the place's link, matched exactly, so whoever approves the call sees
// where it goes and the model must know the place or have looked it up
// (ADR-030). Every answer that describes a team, a channel or a post carries
// its address.

// callRawTool is callTool without reading ids into addresses.
func callRawTool(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := tryRawTool(t, session, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// rawRefused calls a tool without reading ids into addresses, and wants it
// refused, saying each of says.
func rawRefused(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any, says ...string) {
	t.Helper()
	result := callRawTool(t, session, name, arguments)
	if !result.IsError {
		t.Fatalf("%s %v was not refused", name, arguments)
	}
	mustContain(t, name+"'s refusal", errorText(result), says...)
}

func TestAWriteOthersSeeTakesItsPlacesByAddressAlone(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other := seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other)
	channel := seedChannel(t, admin, team, user, other)
	elsewhere := seedChannel(t, admin, team, user, other)
	post := postAs(t, clientAs(t, other), channel.Id, "", "Who reviews the release notes?")
	_, _, err := clientAs(t, user).CreateDirectChannel(t.Context(), user.Id, other.Id)
	check(t, err)
	session, questions := writingSession(t, admin, user, accept)
	address, err := url.Parse(liveURL)
	check(t, err)
	base := "mattermost://" + strings.ToLower(address.Host)

	// Each answer carries the address, made of names.
	var found server.Channel
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "get_channel_info", Arguments: map[string]any{"channel": channel.Id}}), &found)
	var read server.PostWithThread
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "read_post", Arguments: map[string]any{"post_id": post.Id, "include_thread": false}}), &read)
	var inTeam server.Team
	structured(t, callTool(t, session, &mcp.CallToolParams{Name: "get_team_info", Arguments: map[string]any{"team": team.Id}}), &inTeam)
	channelAt := base + "/" + team.Name + "/channels/" + channel.Name
	postAt := channelAt + "/" + post.Id
	if found.Address != channelAt || read.Post == nil || read.Post.Address != postAt || read.Post.ChannelAddress != channelAt || inTeam.Address != base+"/"+team.Name {
		t.Fatalf("the addresses read %q, %+v, %q; want %q, %q", found.Address, read.Post, inTeam.Address, channelAt, postAt)
	}

	// Anything but the address is refused, with where to read it.
	posting := func(arguments map[string]any) map[string]any {
		arguments["message"], arguments["reaches"] = "On it.", members(t, channel.Id)
		return arguments
	}
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": channel.Id}), "is an id", base+"/<team>/channels/<channel>", "get_channel_info")
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": "~" + channel.Name}), "is a name")
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": liveURL + "/" + team.Name + "/channels/" + channel.Name}), "is a link")
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": "https://chat.elsewhere.example/" + team.Name + "/channels/" + channel.Name}),
		"is a link to chat.elsewhere.example", "takes an address", base+"/<team>/channels/<channel>")
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": base + "/" + team.Name + "/channels/" + channel.Id}), "names its channel by id")
	rawRefused(t, session, "create_post", posting(map[string]any{"root_id": base + "/" + team.Name + "/channels/" + elsewhere.Name + "/" + post.Id}), "is not in")
	rawRefused(t, session, "create_post", posting(map[string]any{"root_id": channelAt}), "a channel's address")
	rawRefused(t, session, "create_channel", map[string]any{"team_id": team.Name, "display_name": "Planning"}, "is a name", base+"/<team>")
	noQuestions(t, questions)

	// The address posts, replies, and reaches a direct message.
	posted := createPost(t, session, map[string]any{"channel_id": channelAt, "message": "I can.", "reaches": members(t, channel.Id)})
	replied := createPost(t, session, map[string]any{"root_id": postAt, "message": "Tomorrow morning.", "reaches": members(t, channel.Id)})
	direct := createPost(t, session, map[string]any{"channel_id": base + "/" + team.Name + "/messages/@" + other.Username, "message": "Notes are up.", "reaches": 2})
	if posted.ChannelID != channel.Id || replied.RootID != post.Id || direct.ChannelID == channel.Id || direct.ChannelID == "" {
		t.Errorf("posted %+v, replied %+v, messaged %+v", posted, replied, direct)
	}
	if n := questions.count(); n != 3 {
		t.Errorf("asked %d questions; want one for each post", n)
	}

	// A channel renamed since the model read its address is not found at it.
	_, _, err = admin.PatchChannel(t.Context(), channel.Id, &model.ChannelPatch{Name: model.NewPointer(channel.Name + "-renamed")})
	check(t, err)
	rawRefused(t, session, "create_post", posting(map[string]any{"channel_id": channelAt}), "no channel is at "+channelAt)
}
