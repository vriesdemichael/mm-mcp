---
search:
  boost: 0.3
---

# ADR-035: A press in a view is the person's approval of a proposed post

This is planned with `propose_post`, which replaces `save_draft`; ADR-021, ADR-031 and ADR-033 take it in when it arrives.

`propose_post` lets the person post what the model proposes. It checks the message as a post is checked (ADR-031) and saves it as the person's Mattermost draft where it would go: a channel, a thread, or a direct or group message. The draft is the vessel. Mattermost keeps it in the message box and the drafts list of every client, so a proposal outlives the conversation and the view. In a client that renders views, the view shows the draft as it will be posted, with Post, Edit and Discard (ADR-022). In one that renders none, the answer links the place where the draft waits, for the person to send it from Mattermost.

Pressing Post in the view is the person's answer to the question ADR-021 asks before a post. The press goes through the tool only views call, which the client keeps from its model, and names the draft's version: the update time Mattermost keeps on it. mm-mcp sends the draft only while that version is current, so a press never sends text the person did not see. A draft changed since, by a revision or in Mattermost, refuses the press, and the view draws the new text. Posting deletes the draft, so a second press finds nothing and says the post was sent. Edit writes into the draft, and Discard deletes it while it still holds what was proposed. A message that would notify a whole channel opens Mattermost's own confirmation before it is sent. The model learns what the person did from Mattermost: the draft gone, or the post in the channel.

mm-mcp writes over the person's own words only on their action: a press in the view, or their answer to mm-mcp's question. A draft that holds mm-mcp's own earlier proposal, unchanged, is mm-mcp's to replace. Once the person has typed in it, it is theirs.

The client is trusted to keep the tool only views call from its model, and to press only when the person clicks; mm-mcp cannot tell a click from a script. `MM_MCP_POST_FROM_VIEWS`, true by default, is the lever for a client that cannot be trusted with that: false takes Post out of the view, and the draft waits in Mattermost for the person to send.

A proposal lets the person decide on the text in the conversation without a question their client might answer unseen (ADR-033), and leaves them a draft in Mattermost whatever their client renders.

## Not chosen

- **A token in the result, single-use and bound to the text**: Every part of a result reaches some client's model (ADR-022), and a token fetched from the tool only views call protects nothing that keeping the tool from the model does not.
- **Post through `create_post`, which asks**: No client that renders views shows a question asked during a call from a view.
- **Ask again after the press, as a setting**: For the same reason; the setting takes Post away instead.
- **A view that expires after a few minutes**: The draft's version already refuses a stale press, and a person who reopens a conversation and presses Post on an unchanged draft sends what they see.
- **Set the person's draft aside and restore it once the proposal is posted**: Until then every client of theirs shows the model's text where their words were, and if mm-mcp stops, or they type in that box, their draft is lost.
- **Files in a proposal**: A draft holds only files already uploaded, so a proposal with files would send them to Mattermost before the person agreed; a post with files goes through `create_post`, whose question lists them.
