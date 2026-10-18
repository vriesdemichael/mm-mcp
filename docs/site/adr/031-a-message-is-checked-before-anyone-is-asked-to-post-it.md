---
search:
  boost: 0.3
---

# ADR-031: A message is checked before anyone is asked to post it, and says it was written with AI

Every tool that takes a message the model wrote checks it before the person is asked: `create_post`, `dm`, `group_message`, `update_post` and `save_draft`. A server that does not ask runs the same checks before it acts, since they are mm-mcp's own and not part of the question (ADR-033). A message longer than the server takes, by the `MaxPostSize` it tells its clients, is refused with its length and the limit, so the model shortens it, splits it into replies, or attaches the full text as a file. Its mentions are found as Mattermost finds them: Mattermost's own Markdown parser reads the message, so code of any kind mentions nobody, and its words are split and their trailing punctuation tried off as the server does, so `@here.` at the end of a sentence is `@here` and `_@channel_` is `@channel`. Every mention of a person is resolved against the server's users: one nobody has is refused with the closest usernames, and a deactivated user is named in the question, since they will not be notified. `@channel`, `@all` and `@here` are spelled out in the question with how many people they reach. An edit notifies nobody, Mattermost's own rule, so `update_post` checks its length and none of its mentions, and says so. The question puts what a post does, its mentions, files and marker, before the message, which it shows exactly as it will be posted.

Every post and edit the model writes carries Mattermost's own `ai_generated_by` property, set to the user it was written for, which Mattermost's web app shows as AI-generated beside the post's time. Mattermost accepts the property only for the poster or a bot, so it says who the post is from and that a model wrote it, nothing more. An edit sends the post's properties back with the marker only when the post holds no others: Mattermost sanitises the properties an edit sends, and in its hardened mode refuses some, so a post carrying an integration's properties is edited unmarked, and the question says nothing of a marker. The question says the post will be marked. `MM_MCP_MARK_AI_GENERATED=false` turns the marker off; it is on unless turned off. A draft is not marked: the person sends it, after changing it as they like.

The server's own instructions tell the model how Mattermost renders a message, in a few lines every session reads once, rather than every tool's description repeating it.

A colleague reads a post as the person's own words, so what reaches them should be what the person agreed to, and say plainly when a model wrote it. A mention that names nobody notifies nobody and reads as a mistake; a mention of everyone notifies everyone, which the person should know before they tick the box. A message Mattermost would refuse is better refused before the person spends attention on it.

A write others see states in its call what it does to them, as values mm-mcp reads itself, checked before the call goes further and again just before the write, so a call made on an outdated reading is refused. They are in the call's arguments because that is what a client's own approval shows, a person's prompt or an AI reviewer's, and the model cannot state them without reading the place first. A missing or wrong value is refused with the real one and where it comes from, and the refusal asks the model to weigh whether that is what the person meant.

`create_post` states how far its message goes. `reaches` is the channel's member count, on every post: everyone in the channel can read it, whatever it mentions. `notifies` is required exactly where Mattermost's web app asks before sending, with the number it shows: every member but the sender for `@channel`, `@all` or `@here` in a channel of more than 5 members, while the server's `EnableConfirmNotificationsToChannel` is on and the person has `use_channel_mentions` there; otherwise the members in the channel of the largest user group mentioned, above 5, that the person may mention. The 5 is the web app's own `NOTIFY_ALL_MEMBERS`, which the server does not publish; the rest is read from the server.

`update_post` states `reaches` too, never `notifies`: an edit reaches everyone the post does and notifies nobody. It states the post's whole text as it reads now, `old_message`, beside the new, so the model holds all of what it replaces and the client's approval shows both; a post that reads otherwise, edited since or not all quoted, is refused with what it reads. Once anyone has reacted to the post or replied in the thread it starts, the edit states `responses` as well, how many have, as `read_post` gives it: their answers will read as answers to the new text, which Mattermost tells nobody about, so the refusal names who answered and that a reply in the thread may say the change better.

A deletion states how much goes: `delete_draft` states `discards`, the draft's characters, as `list_drafts` gives its length; `delete_post` states `deletes`, the post and every reply in its thread, other people's included. A refusal quotes what would go.

## Not chosen

- **Split an overlong message into replies on the tool's own initiative**: The person would confirm one post and get several; the model can split it, and each part is then asked about as what it is.
- **Leave the AI marker to the model**: It would be set inconsistently, and a post's provenance is not the model's to decide.
- **Mark drafts**: The person writes the last version of a draft and sends it themselves.
- **Formatting guidance in every tool's description**: It would cost every tool listing in every session for what one paragraph of the server's instructions says.
- **An acknowledgement the model sets, such as `target_audience_ack`**: The model sets it without looking; a number it must read, and that mm-mcp counts, it cannot.
- **`reaches` only above a threshold**: A number to choose, configure and explain, where the count is one read and a refusal hands it over.
- **One number for both**: A message reaches the whole channel whatever it mentions, and only some messages alert everyone at once; Mattermost asks about the second alone.
- **An edit time or another version, to bind a call to what the model read**: It means nothing in a prompt; a draft's length catches all but a rewrite of the same length, and an edit's whole old text catches every change.
- **A count of the characters an edit replaces**: The model would count, or copy the count from a refusal without thinking, and an edit replaces all of a post's text, so the count tells an addition from a rewrite no better than nothing.
- **Only the part an edit changes, old and new**: A change in two places of three would take a long part or several calls, and the model would not hold the whole of what the post said.
- **A label the model gives its edit, such as addition or rewrite**: The model grading its own work, against a threshold between a correction and a rewrite nobody could justify.
