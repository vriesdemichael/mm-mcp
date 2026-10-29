package server

import (
	"encoding/json"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
)

// On a server with an Enterprise Advanced license a person can send a
// burn-on-read post. Each recipient sees it concealed until they reveal it in
// Mattermost, which starts a timer after which it is gone for them. Mattermost
// answers a recipient who has not revealed it with no message, files or
// metadata, but with the rest of its props, so mm-mcp leaves out all it would
// otherwise read from it. No tool reveals one: revealing is a GET, and it
// would burn a post the person never saw.

// concealed says whether the post is concealed from the person: a burn-on-read
// post they did not write and have not revealed, which Mattermost tells by its
// expire_at, as its web app does. A post read for no one is concealed.
func concealed(post *model.Post, self *model.User) bool {
	if post.Type != model.PostTypeBurnOnRead || (self != nil && post.UserId == self.Id) {
		return false
	}
	return post.Metadata == nil || post.Metadata.ExpireAt == 0
}

// burnsAt is when a burn-on-read post is gone for the person, in
// milliseconds: when its timer runs out, once they revealed it, or everyone
// did for its author; until then, the latest it is kept, read or not. Zero
// for any other post.
func burnsAt(post *model.Post) int64 {
	if post.Type != model.PostTypeBurnOnRead {
		return 0
	}
	if post.Metadata != nil && post.Metadata.ExpireAt != 0 {
		return post.Metadata.ExpireAt
	}
	// The props come as JSON: a number, or one written as text.
	switch at := post.GetProp(model.PostPropsExpireAt).(type) {
	case float64:
		return int64(at)
	case int64:
		return at
	case json.Number:
		n, _ := at.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(at, 10, 64)
		return n
	}
	return 0
}
