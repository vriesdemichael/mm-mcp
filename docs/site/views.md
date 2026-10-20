---
title: Views
description: What mm-mcp shows you in the chat, live, and why pressing Post is safe.
---

# Views

Some AI apps can show more than text: small, interactive panels inside the
chat. mm-mcp uses them to show you Mattermost as Mattermost shows it. You see a
thread you asked about, what you missed, and every post your agent makes. A
post your agent drafts for you waits there for you to check, change and send
yourself.

Everything below is live. It is the same panel mm-mcp shows in your AI app,
playing back a recorded conversation from a test Mattermost. Press anything:
nothing is sent from this page, and **Start over** puts a panel back as it was.

!!! tip "Does my app show them?"

    Goose's desktop app does. VS Code does once `chat.mcp.apps.enabled` is on
    in its settings. Ask your agent to run `diagnose` and it tells you whether
    yours does. In an app that shows no panels, everything still works: your
    agent answers in words, and a post it drafts waits in Mattermost for you
    ([more below](#if-your-app-shows-no-panels)).

## Check and send a post your agent drafts

Ask your agent to draft something for you, and nothing is posted until you
say so. The draft waits in Mattermost's message box, as your own unsent
draft, and the panel shows it exactly as it will look once posted.

<div class="mm-chat mm-chat--wide" markdown>
<div class="mm-chat__bar"><span class="mm-chat__dot"></span>Your agent, with mm-mcp</div>
<div class="mm-chat__body">
<div class="mm-msg mm-msg--you">Jordan asks whether the migration is done. Draft a reply for me to check.</div>
<div class="mm-tool">propose_post, a reply in ~release-planning</div>
<div class="mm-view" data-view="proposal" data-title="A drafted reply, as it will be posted"></div>
</div>
</div>

<p class="mm-try">Try it: press <strong>Edit</strong>, change a word, save,
then press <strong>Post</strong>.</p>

- **Post** sends it under your name, marked as written with AI, and clears
  the draft.
- **Edit** changes the draft. Once you have changed it, it is yours: your
  agent will not replace it.
- **Discard** deletes the draft.
- A link shows where it really goes, next to the words it is written as.

If you change the draft in Mattermost while the panel is open, the panel
notices. Your next press shows you the new text instead of sending the old one.

### A message to the whole channel

Mattermost asks before a message with `@channel`, `@all` or `@here` notifies
more than five people. The panel asks the same question, in Mattermost's own
words, with the number of people it will notify.

<div class="mm-view" data-view="confirm" data-title="A draft that notifies the whole channel"></div>

<p class="mm-try">Try it: press <strong>Post</strong>, then
<strong>Cancel</strong> or <strong>Confirm</strong>.</p>

## When you already started a message there

Mattermost keeps one unsent draft in each channel and in each thread. If
yours is already there, your agent never writes over it:

- **You asked your agent to improve it:** the panel shows the suggested
  changes, word by word, with **Apply** and **Keep mine**.
- **Otherwise:** the panel shows your draft next to the suggestion, with
  **Replace mine** and **Keep mine**.

Nothing is changed until you press.

<div class="grid" markdown>
<div class="mm-view" data-view="revision" data-title="Suggested changes to your draft"></div>
<div class="mm-view" data-view="choice" data-title="Your draft next to a suggestion"></div>
</div>

## See a thread

Ask to see a thread and your agent shows it with
[`show`](tools.md#show). You get pictures, formatting, reactions, custom
emoji, and images you can open full size. A long thread loads more as you
ask.

<div class="mm-view" data-view="thread" data-title="A thread"></div>

<p class="mm-try">Try it: press <strong>2 more replies</strong>, or the chart
to see it full size.</p>

## See what you missed

Ask what you missed, and the panel lists:

- your channels with unread messages, by team;
- the posts that mention you;
- the threads you follow that have new replies.

Open a channel right there to read it. Where your agent may post,
**Mark read** marks it read in Mattermost too.

<div class="mm-view" data-view="unreads" data-title="What you missed"></div>

<p class="mm-try">Try it: press <strong>Design</strong> to open it, then
<strong>Mark read</strong>.</p>

## See what your agent posted

When your agent posts for you with [`create_post`](tools.md#create_post),
[`dm`](tools.md#dm) or [`group_message`](tools.md#group_message), the post
shows in the chat as it was posted, and where. An agent that posts in many
places cannot do so out of your sight.

<div class="mm-view" data-view="posted" data-title="A post your agent made"></div>

## If your app shows no panels

Every app still works. A post your agent drafts waits in Mattermost's message
box in every Mattermost app you use, and your agent's answer links to
the place where it waits. Send it, change it or clear it there.

<div class="mm-box" role="img" aria-label="Mattermost's message box in ~release-planning, with the drafted reply waiting in it, under Sam's post it replies to.">
<div class="mm-box__head">~release-planning</div>
<div class="mm-box__post"><strong>Sam Okafor</strong> The 2.4 release moves to Thursday. Anything blocking, say so here.</div>
<div class="mm-box__input">Thanks @jordan, the migration finished at 09:40. The steps are in [the runbook](https://docs.example.com/runbook-2.4).<span class="mm-box__send">Send</span></div>
<div class="mm-box__note">Draft</div>
</div>

## Which apps show panels

| App | Panels | Good to know |
|---|---|---|
| Goose, the desktop app | Yes | In its "approve" and "smart approve" modes, Goose blocks every button in a panel. The panel then says so and offers to open Mattermost instead. |
| VS Code | Yes, with `chat.mcp.apps.enabled` on | VS Code asks you before your agent drafts a post, as it does before anything that writes. Your own press in a panel is not asked about again. |
| Claude desktop, Claude on the web | Not confirmed | These apps cannot be tested by machine. A check by hand is recorded on [mm-mcp#37](https://github.com/vriesdemichael/mm-mcp/issues/37). |
| An app without panels | No | Your agent answers in words, and drafts wait in Mattermost. |

Goose and VS Code are tested with every change to mm-mcp.

## Questions people ask

**Can my agent press Post for me?**
: No. The buttons in a panel go through a tool that only the panel can use:
  your AI app keeps it away from the agent. Your app is trusted to do that,
  and to press only when you click. mm-mcp cannot tell a click from a script,
  so where that trust is not there, an administrator can take Post out of the
  panel with `MM_MCP_POST_FROM_VIEWS=false`
  ([Configuration](configuration.md)). The draft then waits in Mattermost for
  you to send.

**Will people see that AI wrote my post?**
: A post you send with **Post** in the panel is marked as written with AI, as
  every post mm-mcp makes is; Mattermost shows that next to it. A draft you
  send from Mattermost yourself is not marked: it is yours. An administrator
  can turn the mark off with `MM_MCP_MARK_AI_GENERATED=false`.

**Can it send something different from what I see?**
: No. Post sends the text the panel shows, and only while your draft still
  holds exactly that. If the draft changed, or you attached a file to it in
  Mattermost, the panel shows you what it holds now instead of sending. The
  preview hides nothing: a link shows where it leads, and a character you
  cannot see, or one that turns the text's direction, is flagged. Here, the
  name `invoice_fdp.exe` is made to read as `invoice_exe.pdf`:

<div class="mm-view" data-view="preview" data-no-presses data-title="A draft whose text hides an executable"></div>

**Does my agent know what I did?**
: Your app may pass it on: the panel tells it, in a fixed sentence, whether you
  posted, changed or discarded the draft. Not every app passes that on, so
  your agent can also check Mattermost: the draft gone, or the post in the
  channel.

**Does the panel send my data anywhere?**
: No. The panel reaches no network. What it shows comes from mm-mcp through
  your app, pictures included, and mm-mcp talks to nothing but your Mattermost
  ([Security](security.md)).

??? info "For the technically curious: what happens when you press Post"

    ```mermaid
    sequenceDiagram
        actor You
        participant App as Your AI app
        participant Model as The AI model
        participant mm as mm-mcp
        participant MM as Mattermost
        Model->>mm: propose_post, the message
        mm->>MM: save it as your draft, marked as mm-mcp's proposal
        mm-->>App: the draft
        App->>You: the panel: the post as it will be posted
        You->>App: Post
        App->>mm: view_change, post, the text the panel shows
        mm->>MM: does the draft still hold that text, and nothing else?
        mm->>MM: post it, marked as written with AI, and delete the draft
        mm-->>App: posted
    ```

    The panel is an [MCP Apps](https://github.com/modelcontextprotocol/ext-apps)
    view: a page that ships inside mm-mcp, which your app runs in a sandbox
    without network access. Its buttons call `view_change`, a tool marked for
    views alone, which mm-mcp offers only to an app that says it shows views,
    and only once writes are allowed. The decisions behind this are in
    [ADR-022](adr/022-mcp-apps-views-on-show-and-on-every-post.md) and
    [ADR-035](adr/035-a-proposed-post-waits-as-the-persons-draft.md).
