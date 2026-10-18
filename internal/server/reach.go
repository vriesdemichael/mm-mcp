package server

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// How far a message goes, stated in the call and checked against mm-mcp's own
// count before anything is posted (ADR-031). The count is in the call's
// arguments because that is what a client's approval shows, a person's prompt
// or an AI reviewer's, and the model cannot state it without looking.
//
// Two numbers, as Mattermost has two: reaches, everyone in the channel, who
// can read the message whatever it mentions; and notifies, the people alerted
// at once, exactly where Mattermost's web app asks before sending.

// notifyAllMembers is the Mattermost web app's NOTIFY_ALL_MEMBERS: it asks
// before sending a message that would alert a channel, or a group in it, of
// more members than this. The server does not publish the number; the web
// app's use_submit.tsx and use_groups.tsx hold it.
const notifyAllMembers = 5

// reachField is the argument of a tool that posts or edits in a channel.
type reachField struct {
	Reaches *int64 `json:"reaches,omitempty" jsonschema:"how many members the channel the message is in has, as get_channel_stats counts them: everyone who can read it, whether it mentions them or not. Required; a missing or wrong count is refused with the real one"`
}

// notifyField is the argument of a tool that posts a new message in a channel.
type notifyField struct {
	Notifies *int64 `json:"notifies,omitempty" jsonschema:"how many people the message alerts at once, where Mattermost would ask before sending it: with @channel, @all or @here in a channel of more than 5 members, its members but the user; with a user group's mention, that group's members in the channel. Leave it out otherwise; a missing or wrong number is refused with the real one"`
}

// stating is the input of a tool whose call states how far its message goes.
type stating interface {
	stated() (reaches, notifies *int64)
}

// checkReaches refuses a call whose reaches is not the channel's member count.
func checkReaches(tool, where string, stated *int64, members int64) error {
	if stated != nil && *stated == members {
		return nil
	}
	return fmt.Errorf("%s did not run: a message in %s reaches %s, the channel's members as Mattermost counts them, and the call %s. "+
		"Consider whether the person meant this message for all of them. If they did, call again with reaches: %d. If not, "+
		"send it somewhere narrower, such as a smaller channel or a direct or group message, or ask them where it should go",
		tool, where, peopleCount(members), said(stated), members)
}

// checkNotifies refuses a call whose notifies is not what Mattermost would ask
// about, or that states a number where Mattermost would ask nothing.
func checkNotifies(tool string, stated *int64, loud loudness) error {
	switch {
	case loud.count == 0 && (stated == nil || *stated == 0):
		return nil
	case loud.count == 0:
		return fmt.Errorf("%s did not run: the call says notifies: %d, but this message alerts nobody at once the way Mattermost asks about. Leave notifies out", tool, *stated)
	case stated != nil && *stated == loud.count:
		return nil
	}
	return fmt.Errorf("%s did not run: %s, which Mattermost asks about before sending, and the call %s. "+
		"Consider whether the person meant to alert all of them. If they did, call again with notifies: %d. If not, "+
		"mention only the people meant, post somewhere narrower, or ask them",
		tool, loud.why, said(stated), loud.count)
}

// said is what a call stated, for a refusal.
func said(stated *int64) string {
	if stated == nil {
		return "left the number out"
	}
	return fmt.Sprintf("said %d", *stated)
}

// loudness is what Mattermost's web app would ask about before sending a
// message: how many people it alerts at once, and why, or nothing.
type loudness struct {
	count int64
	why   string
}

// groupMention is a user group a message mentions, and how many of its
// members are in the channel.
type groupMention struct {
	word    string
	name    string
	members int64
	allowed bool
}

// loudFacts is what the web app reads to decide whether to ask.
type loudFacts struct {
	confirming bool
	members    int64
	specials   []string
	// mayMentionChannel is the user's use_channel_mentions in the channel.
	mayMentionChannel bool
	groups            []groupMention
	where             string
}

// loudnessOf decides as the web app's use_submit.tsx does. @channel, @all and
// @here alert every member but the sender, where the server asks for
// confirmation, the user may mention the channel, and it has more than
// notifyAllMembers members. Without them, a group mention alerts the group's
// members in the channel, where there are more than notifyAllMembers, and the
// largest such group is the count.
func loudnessOf(f loudFacts) loudness {
	if !f.confirming {
		return loudness{}
	}
	if len(f.specials) > 0 {
		if !f.mayMentionChannel || f.members <= notifyAllMembers {
			return loudness{}
		}
		return loudness{
			count: f.members - 1,
			why:   fmt.Sprintf("%s alerts %s, every member of %s but the user", strings.Join(f.specials, ", "), peopleCount(f.members-1), f.where),
		}
	}
	var loudest loudness
	for _, group := range f.groups {
		if group.allowed && group.members > notifyAllMembers && group.members > loudest.count {
			loudest = loudness{
				count: group.members,
				why:   fmt.Sprintf("%s alerts %s, the members of the group %s in %s", group.word, peopleCount(group.members), group.name, f.where),
			}
		}
	}
	return loudest
}

// loudnessIn reads what loudnessOf decides on, for a message posted in
// channel, which checkMessage found the mentions of. GetClientConfig,
// GetChannelMember and GetRolesByNames; for a mention of a user group,
// GetChannelMemberCountsByGroup too.
func loudnessIn(ctx context.Context, client *mattermost.Client, self *model.User, channel *model.Channel, where string, checked checkedMessage, members int64) (loudness, error) {
	config, err := client.ClientConfig(ctx)
	if err != nil {
		return loudness{}, err
	}
	facts := loudFacts{confirming: config["EnableConfirmNotificationsToChannel"] == "true", members: members, specials: checked.specials, where: where}
	// Only what loudnessOf would look at is read.
	everyone := facts.confirming && len(facts.specials) > 0 && members > notifyAllMembers
	if everyone {
		if facts.mayMentionChannel, err = allowedIn(ctx, client, self, channel, model.PermissionUseChannelMentions.Id); err != nil {
			return loudness{}, err
		}
	}
	if facts.confirming && len(facts.specials) == 0 && len(checked.groups) > 0 {
		if facts.groups, err = groupsIn(ctx, client, self, channel, config, checked.groups); err != nil {
			return loudness{}, err
		}
	}
	return loudnessOf(facts), nil
}

// groupsIn is each user group a message mentions, with its members in the
// channel and whether the user may mention it there. Groups are a licensed
// edition's feature, so Team Edition never gets here.
func groupsIn(ctx context.Context, client *mattermost.Client, self *model.User, channel *model.Channel, config map[string]string, mentioned []mentionedGroup) ([]groupMention, error) {
	may, err := allowedIn(ctx, client, self, channel, model.PermissionUseGroupMentions.Id)
	if err != nil {
		return nil, err
	}
	counts, err := client.ChannelMemberCountsByGroup(ctx, channel.Id)
	if err != nil {
		return nil, err
	}
	return groupMentions(mentioned, config, may, counts), nil
}

// groupMentions is each mentioned group with its members in the channel, and
// whether the user may mention it: it allows it, its kind is on, and they may
// mention groups there. Mattermost answers a lookup of groups only where its
// licence covers them, so an LDAP group it answered with is on; a custom group
// is on while the server's EnableCustomGroups is, as the web app reads it.
func groupMentions(mentioned []mentionedGroup, config map[string]string, may bool, counts []*model.ChannelMemberCountByGroup) []groupMention {
	custom := config["EnableCustomGroups"] == "true"
	inChannel := map[string]int64{}
	for _, count := range counts {
		inChannel[count.GroupId] = count.ChannelMemberCount
	}
	out := make([]groupMention, 0, len(mentioned))
	for _, m := range mentioned {
		kindOn := m.group.Source == model.GroupSourceLdap || (m.group.Source == model.GroupSourceCustom && custom)
		out = append(out, groupMention{word: m.word, name: m.group.DisplayName, members: inChannel[m.group.Id], allowed: may && kindOn && m.group.AllowReference})
	}
	return out
}

// allowedIn reports whether the user holds a permission of the channel's, as
// the web app works it out: from the roles they hold on the system and in the
// channel, whose names Mattermost already gives by the team's and channel's
// schemes. GetChannelMember, GetRolesByNames.
func allowedIn(ctx context.Context, client *mattermost.Client, self *model.User, channel *model.Channel, permission string) (bool, error) {
	names := strings.Fields(self.Roles)
	member, err := client.Membership(ctx, channel.Id, self.Id)
	if err != nil {
		return false, err
	}
	names = append(names, strings.Fields(member.Roles)...)
	roles, err := client.RolesByNames(ctx, names)
	if err != nil {
		return false, err
	}
	return permitted(roles, permission), nil
}

// permitted reports whether any of the roles grants the permission.
func permitted(roles []*model.Role, permission string) bool {
	return slices.ContainsFunc(roles, func(role *model.Role) bool { return slices.Contains(role.Permissions, permission) })
}

// checkImpact holds a call that states how far its message goes to mm-mcp's
// own count, read as the call runs, so a count read before members joined is
// refused. A tool whose input states nothing is not held to it.
func checkImpact(ctx context.Context, client *mattermost.Client, tool string, where destination, checked checkedMessage, input any) error {
	s, ok := input.(stating)
	if !ok {
		return nil
	}
	reaches, notifies := s.stated()
	members, err := where.size(ctx)
	if err != nil {
		return err
	}
	if err := checkReaches(tool, where.label, reaches, members); err != nil {
		return err
	}
	loud, err := loudnessIn(ctx, client, where.self, where.target, where.label, checked, members)
	if err != nil {
		return err
	}
	return checkNotifies(tool, notifies, loud)
}

// impactUses are the operations checkImpact calls beyond checkMessage's.
func impactUses() []Use {
	return []Use{
		{
			Operation: "GetChannelMember",
			Params: map[string]Coverage{
				"channel_id": Fixed("the post's channel", "the user's roles there say whether they may mention the whole channel"),
				"user_id":    Fixed("me", "the post goes out under the user's name, with their permissions"),
			},
		},
		{Operation: "GetRolesByNames", Params: map[string]Coverage{}},
		{
			Operation: "GetChannelMemberCountsByGroup",
			Params: map[string]Coverage{
				"channel_id":        Fixed("the post's channel", "a group mention alerts the group's members in it"),
				"include_timezones": Fixed("false", "the refusal counts people, not their timezones"),
			},
			Licensed: "asked only once a message mentions a user group, which Team Edition has none of",
		},
	}
}

// reachCheck and notifyCheck say what reaches and notifies are checked
// against, for the specs that take them.
const (
	reachCheck  = "the member count of the channel the message is in, from its channel stats"
	notifyCheck = "what Mattermost's web app would ask about before sending: everyone but the user for @channel, @all and @here, or a user group's members in the channel, above 5"
)
