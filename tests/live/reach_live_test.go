//go:build live

package live

import (
	"strconv"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// How far a message goes, stated in the call and checked against mm-mcp's own
// count: reaches, the channel's members, on every post and edit in a channel;
// notifies, where Mattermost's web app would ask before sending (ADR-031).

// refusedSaying calls a tool and wants it refused, its refusal saying each of
// says.
func refusedSaying(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any, says ...string) {
	t.Helper()
	result := callTool(t, session, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if !result.IsError {
		t.Fatalf("%s %v was not refused", name, arguments)
	}
	mustContain(t, name+"'s refusal", errorText(result), says...)
}

func TestAPostAndAnEditStateHowManyTheyReach(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other, third, late := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other, third, late)
	channel := seedChannel(t, admin, team, user, other, third)
	session, questions := writingSession(t, admin, user, accept)
	count := members(t, channel.Id)
	reach := count

	refusedSaying(t, session, "create_post", map[string]any{"channel_id": channel.Id, "message": "Standup moved to 10"},
		peopleIn(count), "~"+channel.DisplayName, "left the number out", "Consider whether", "reaches: "+itoa(count))
	refusedSaying(t, session, "create_post", map[string]any{"channel_id": channel.Id, "message": "Standup moved to 10", "reaches": count - 1},
		"said "+itoa(count-1), "reaches: "+itoa(count))
	noQuestions(t, questions)
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Fatalf("a refused post was posted: %d posts", len(posts))
	}

	posted := createPost(t, session, map[string]any{"channel_id": channel.Id, "message": "Standup moved to 10", "reaches": reach})
	questions.only(t)

	// A count read before someone joined is out of date, and refused with the new one.
	_, _, err := admin.AddChannelMember(t.Context(), channel.Id, late.Id)
	check(t, err)
	refusedSaying(t, session, "update_post", map[string]any{"post_id": posted.ID, "message": "Standup moved to 10:30", "reaches": reach},
		"said "+itoa(reach), "reaches: "+itoa(reach+1))
	refusedSaying(t, session, "update_post", map[string]any{"post_id": posted.ID, "message": "Standup moved to 10:30"},
		"left the number out")
	callTool(t, session, &mcp.CallToolParams{Name: "update_post", Arguments: map[string]any{
		"post_id": posted.ID, "message": "Standup moved to 10:30", "reaches": reach + 1, "old_message": "Standup moved to 10",
	}})
	stored, _, err := admin.GetPost(t.Context(), posted.ID, "")
	check(t, err)
	if stored.Message != "Standup moved to 10:30" {
		t.Errorf("the post reads %q", stored.Message)
	}
}

// Members who join while the person is asked change how far the post goes, so
// it is counted again before it is posted.
func TestAPostIsCountedAgainAfterThePersonIsAsked(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other, late := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other, late)
	channel := seedChannel(t, admin, team, user, other)
	count := members(t, channel.Id)
	session := answeringAfter(t, admin, user, func() {
		if _, _, err := admin.AddChannelMember(t.Context(), channel.Id, late.Id); err != nil {
			t.Error(err)
		}
	}, accept)

	refusedSaying(t, session, "create_post", map[string]any{"channel_id": channel.Id, "message": "Who is in?", "reaches": count},
		"said "+itoa(count), "reaches: "+itoa(count+1))
	if posts := messagesIn(t, admin, channel.Id); len(posts) != 0 {
		t.Fatalf("the post went out to members nobody counted: %d posts", len(posts))
	}
}

func TestAMessageThatAlertsAWholeChannelStatesWhomItNotifies(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	config, _, err := admin.GetConfig(t.Context())
	check(t, err)
	if !*config.TeamSettings.EnableConfirmNotificationsToChannel {
		t.Fatal("the instance does not confirm notifications to a channel, which is Mattermost's default")
	}
	user := seedUser(t, admin)
	crowd := []*model.User{user}
	for range 6 {
		crowd = append(crowd, seedUser(t, admin))
	}
	team := seedTeam(t, admin, crowd...)
	big := seedChannel(t, admin, team, crowd...)
	small := seedChannel(t, admin, team, crowd[:4]...)
	session, questions := writingSession(t, admin, user, accept)
	count := members(t, big.Id)
	if count <= 5 {
		t.Fatalf("the channel has %d members; it must have more than Mattermost's 5", count)
	}

	refusedSaying(t, session, "create_post", map[string]any{"channel_id": big.Id, "message": "@channel the release is out", "reaches": count},
		"@channel alerts "+peopleIn(count-1), "~"+big.DisplayName, "left the number out", "Consider whether", "notifies: "+itoa(count-1))
	refusedSaying(t, session, "create_post", map[string]any{"channel_id": big.Id, "message": "@here @all the release is out", "reaches": count, "notifies": count},
		"said "+itoa(count), "notifies: "+itoa(count-1))
	refusedSaying(t, session, "create_post", map[string]any{"channel_id": big.Id, "message": "the release is out", "reaches": count, "notifies": 3},
		"Leave notifies out")
	noQuestions(t, questions)

	posted := createPost(t, session, map[string]any{"channel_id": big.Id, "message": "@channel the release is out", "reaches": count, "notifies": count - 1})
	mustContain(t, "question", questions.only(t).Message, "@channel notifies everyone in the channel")

	// Not in a channel of 5 or fewer, and never for an edit, which notifies nobody.
	createPost(t, session, map[string]any{"channel_id": small.Id, "message": "@here quick sync?"})
	callTool(t, session, &mcp.CallToolParams{Name: "update_post", Arguments: map[string]any{
		"post_id": posted.ID, "message": "@channel the release is out, notes in the thread", "reaches": count, "old_message": "@channel the release is out",
	}})
	if n := len(messagesIn(t, admin, big.Id)) + len(messagesIn(t, admin, small.Id)); n != 2 {
		t.Errorf("the channels hold %d posts; want the two that were stated right", n)
	}
}

func peopleIn(n int64) string {
	if n == 1 {
		return "1 person"
	}
	return itoa(n) + " people"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// An edit is counted again after the person is asked, as a post is.
func TestAnEditIsCountedAgainAfterThePersonIsAsked(t *testing.T) {
	t.Parallel()
	admin := admin(t)
	user, other, late := seedUser(t, admin), seedUser(t, admin), seedUser(t, admin)
	team := seedTeam(t, admin, user, other, late)
	channel := seedChannel(t, admin, team, user, other)
	post := postAs(t, clientAs(t, user), channel.Id, "", "Retro at four")
	count := members(t, channel.Id)
	session := answeringAfter(t, admin, user, func() {
		if _, _, err := admin.AddChannelMember(t.Context(), channel.Id, late.Id); err != nil {
			t.Error(err)
		}
	}, accept)

	refusedSaying(t, session, "update_post", map[string]any{"post_id": post.Id, "message": "Retro at five", "reaches": count, "old_message": "Retro at four"},
		"said "+itoa(count), "reaches: "+itoa(count+1))
	stored, _, err := admin.GetPost(t.Context(), post.Id, "")
	check(t, err)
	if stored.Message != "Retro at four" {
		t.Errorf("the edit went out to members nobody counted: %q", stored.Message)
	}
}
