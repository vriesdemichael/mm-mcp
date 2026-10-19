---
search:
  boost: 0.3
---

# ADR-021: Read-only by default

A server starts read-only. A tool that changes anything in Mattermost is registered only when `MM_MCP_ALLOW_WRITES` is true, so a read-only server does not list it and a client cannot call it. Which tools are listed never varies by what a client says it can do, but for the tools only views call (ADR-022).

Once writes are allowed, a write is one of two kinds. A change other people see: posting, replying, editing, deleting, reacting, pinning, uploading, joining or leaving a channel, adding people to one, creating one, and setting a status. What mm-mcp does for such a change, and who decides whether it runs, is ADR-033's. A change that is the user's alone, or gone within seconds: following a thread, saving a post, proposing a post as a draft only they see, marking a channel read when they ask, or showing that they are typing. Such a tool names why in its spec's `Unasked`, may not be destructive, and never asks, whatever the settings: a question for each of these would teach the person to tick a box without reading. What the person presses in a view goes through the tool only views call that changes something, which never asks either (ADR-022): it may post, or replace or delete a draft, since the press is the person's own (ADR-035).

What the person has read is not the agent's to change. Mattermost's unread counts, read marks and "mark as unread" record whether a person read something, and a model ingesting a channel is not the person reading it. Reading never marks anything read as a side effect, and no tool marks anything unread. One tool, `mark_channel_read`, marks a channel read, and its description holds the model to doing so only when the person asks: a person triaging through an agent says which channels they are done with, and badges they cannot clear without opening each channel themselves would make the agent's catch-up useless. It changes only the person's own record.

Every tool declares a title and all four hints, each set by what the MCP specification defines it to mean. openWorldHint is false throughout. The specification sets it true for a tool that may reach an open world of external entities, such as a web search, and false for a closed domain, such as a memory tool, whatever text that holds; one configured Mattermost server is a closed domain, though people write anything into it. Whether a tool changes Mattermost is its readOnlyHint, and nothing else: `server.Exposed` offers a tool not annotated read-only only when writes are allowed. The hints are hints, which a client may act on or ignore; some run a read-only tool without asking anyone, so a tool is annotated read-only only when it changes nothing. A tool that writes files on the person's machine is offered on its own terms (ADR-029). The governance tests hold all of this (ADR-015).

Annotate a tool that changes Mattermost as not read-only, which keeps it out of a read-only server through `server.Exposed`. When others see the change, follow ADR-033. When only the user sees it, or it lasts seconds, mark the spec `unasked` with the reason.

The credential may be a person's own session or token, on their employer's server, and a post appears under their name to colleagues. Reading is what most requests need, and writing is the exception that someone turns on. A tool that is missing because writes are off is visible in the configuration; a write made where nobody expected writes is not visible until a colleague reads it. A followed thread or a saved post is visible to no one else, and a typing indicator is gone before anyone could act on it.

## Not chosen

- **Ask for every change, the user's own included**: A question for each followed thread or typing indicator wears out the attention a question for a post needs.
- **Mark what the model read as read**: The unread count is the person's record of what they have seen; the model reading a channel for them does not mean they have.
- **List the tools a client can use**: What a client says it can do is a claim, and the same client lists differently from one mode to another; a server's list is its configuration's.
