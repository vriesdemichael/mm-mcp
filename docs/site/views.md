---
title: Views
description: What mm-mcp shows you in the chat, live, and why pressing Post is safe.
---

# Views

Some AI apps can show more than text: small, interactive panels inside the
chat, which MCP calls views. mm-mcp uses views to show you Mattermost as
Mattermost shows it. You see a thread you asked about, what you missed, and
every post your agent makes. A post your agent drafts for you waits there for
you to check, change and send yourself.

Everything below is live. Each view is the same one mm-mcp shows in your AI
app, playing back a recorded conversation from a test Mattermost. Press
anything: nothing is sent from this page, and **Start over** puts a view back
as it was.

!!! tip "Does my app show views, and what do I need?"

    VS Code shows them once `chat.mcp.apps.enabled` is on in its settings.
    Claude Code shows them in the Claude desktop app, to read. Ask your agent
    to run `diagnose` and it tells you whether yours does
    ([Installation](installation.md)).

    Seeing a thread, or what you missed, works as soon as mm-mcp is set up.
    Drafting a post, and **Post**, **Edit**, **Discard** and **Mark read** in
    a view, need writes allowed, with `MM_MCP_ALLOW_WRITES=true`
    ([Configuration](configuration.md)).

    In an app that shows no views, everything still works: your agent answers
    in words, and a post it drafts waits in Mattermost for you
    ([more below](#if-your-app-shows-no-views)).

## Check and send a post your agent drafts

Ask your agent to draft something for you, and nothing is posted until you
say so. The draft waits in Mattermost's message box, as your own unsent
draft, and the view shows it exactly as it will look once posted.

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

If you change the draft in Mattermost while the view is open, the view
notices, and leaves the draft to you there instead of sending the old text. If
your agent proposes again, your next press shows you the new text first.

### A message to the whole channel

Mattermost asks before a message with `@channel`, `@all` or `@here` notifies
more than five people. The view asks too, with the number of people it will
notify, counted as Mattermost counts them.

<div class="mm-view" data-view="confirm" data-title="A draft that notifies the whole channel"></div>

<p class="mm-try">Try it: press <strong>Post</strong>, then
<strong>Cancel</strong> or <strong>Post anyway</strong>.</p>

## When you already started a message there

Mattermost keeps one unsent draft in each channel and in each thread. If
yours is already there, your agent never writes over it:

- **You asked your agent to improve it:** the view shows the suggested
  changes, word by word, with **Apply** and **Keep mine**.
- **Otherwise:** the view shows your draft next to the suggestion, with
  **Replace mine** and **Keep mine**.

Nothing is changed until you press.

<div class="grid" markdown>
<div class="mm-view" data-view="revision" data-title="Suggested changes to your draft"></div>
<div class="mm-view" data-view="choice" data-title="Your draft next to a suggestion"></div>
</div>

## See a thread

Ask to see a thread and your agent shows it with
[`show`](tools.md#show). You get pictures, formatting, reactions, custom
emoji, images you can open full size, and the cards a webhook or bot posts,
such as an alert with its red or green bar, and the blocks an integration
posts on Mattermost 11.11 and later, with their columns and the sections you
unfold. A link to another post shows that post quoted below it, a long one cut
short with **Show more** for the rest: press the quote to read the post, in the
view where your app allows it and otherwise in Mattermost. A link to a web page shows the page's card. A picture a link or a
card leads to is named, and opens in your browser: the view itself reaches
nothing outside Mattermost. A post marked Important or
Urgent carries its label, and one that asks to be acknowledged says how many
did, with who when you point at it. A burn-on-read message you have not
opened stays concealed, as in Mattermost: **View message in Mattermost**
takes you there to open it. A call shows when it started and by whom, or
when it ended, how long it lasted and who was in it. A long thread loads more
as you ask.

<div class="mm-view" data-view="thread" data-title="A thread"></div>

<p class="mm-try">Try it: press <strong>2 more replies</strong>, or the chart
to see it full size.</p>

## See what you missed

Ask what you missed, and the view lists:

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

## If your app shows no views

Every app still works. A post your agent drafts waits in Mattermost's message
box in every Mattermost app you use, and your agent's answer links to
the place where it waits. Send it, change it or clear it there.

<div class="mm-box" role="img" aria-label="A sketch of the message box in Mattermost, in ~release-planning, with the drafted reply waiting in it, under Sam's post it replies to.">
<div class="mm-box__head">~release-planning</div>
<div class="mm-box__post"><strong>Sam Okafor</strong> The 2.4 release moves to Thursday. Anything blocking, say so here.</div>
<div class="mm-box__input">Thanks @jordan, the migration finished at 09:40. The steps are in [the runbook](https://docs.example.com/runbook-2.4).<span class="mm-box__send">Send</span></div>
<div class="mm-box__note">Draft</div>
</div>

## Which apps show views

| App | Views | Good to know |
|---|---|---|
| VS Code | Yes, with `chat.mcp.apps.enabled` on | VS Code asks you before your agent drafts a post, as it does before anything that writes. Your own press in a view is not asked about again. |
| Claude Code, in the Claude desktop app | Yes, to read | Threads and what you missed show, and on an `https` server a link opens after a confirmation. As of Claude Code 2.1.309 it passes nothing from a view on to mm-mcp, so people show as initials and pictures as their names, and a draft waits in Mattermost for you to send, with no Post, Edit or Discard. In a terminal, Claude Code shows text. |
| Codex | Not checked | Nobody has tried it yet. In an app that shows no views, everything still works in words. |
| Goose, the desktop app | Yes | In its "approve" and "smart approve" modes, Goose blocks every button in a view. The view then says so and offers to open Mattermost instead. |
| Claude desktop, Claude on the web | Not confirmed | These apps cannot be tested by machine. A check by hand is recorded on [mm-mcp#37](https://github.com/vriesdemichael/mm-mcp/issues/37). |
| An app without views | No | Your agent answers in words, and drafts wait in Mattermost. |

VS Code and Goose are tested with every change to mm-mcp, because a machine can drive them.

## Questions people ask

**Can my agent press Post for me?**
: No. The buttons in a view go through a tool that only the view can use:
  your AI app keeps it away from the agent. Your app is trusted to do that,
  and to press only when you click. mm-mcp cannot tell a click from a script,
  so where that trust is not there, an administrator can take Post out of the
  view with `MM_MCP_POST_FROM_VIEWS=false`
  ([Configuration](configuration.md)). The draft then waits in Mattermost for
  you to send.

**Will people see that AI wrote my post?**
: A post you send with **Post** in the view is marked as written with AI, as
  every post mm-mcp makes is; Mattermost shows that next to it. A draft you
  send from Mattermost yourself is not marked: it is yours. An administrator
  can turn the mark off with `MM_MCP_MARK_AI_GENERATED=false`.

**Can it send something different from what I see?**
: No. Post sends the text the view shows, and only while your draft still
  holds exactly that, as the view put it there. If you typed in the draft, or
  attached a file to it, in Mattermost, the view leaves it to you to send from
  there; if your agent proposed again, the view shows you the new text instead
  of sending. The preview hides nothing: a link shows where it leads, a link
  definition Mattermost does not show is shown, and a character you cannot
  see, or one that turns the text's direction, is flagged, even when it is
  written as its number. Here, the
  name `invoice_fdp.exe` is made to read as `invoice_exe.pdf`:

<div class="mm-view" data-view="preview" data-no-presses data-title="A draft whose text hides an executable"></div>

**Does my agent know what I did?**
: Your app may pass it on: the view tells it, in a fixed sentence, whether you
  posted, changed or discarded the draft. Not every app passes that on, so
  your agent can also check Mattermost: the draft gone, or the post in the
  channel.

**Does the view send my data anywhere?**
: No. The view reaches no network. What it shows comes from mm-mcp through
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
        App->>You: the view: the post as it will be posted
        You->>App: Post
        App->>mm: view_change, post, the text the view shows
        mm->>MM: does the draft still hold that text, and nothing else?
        mm->>MM: post it, marked as written with AI, and delete the draft
        mm-->>App: posted
    ```

    Technically, a view is an [MCP Apps](https://github.com/modelcontextprotocol/ext-apps)
    view: a page that ships inside mm-mcp, which your app runs in a sandbox
    without network access. Its buttons call `view_change`, a tool marked for
    views alone, which mm-mcp offers only to an app that says it shows views,
    and only once writes are allowed. The decisions behind this are in
    [ADR-022](adr/022-mcp-apps-views-on-show-and-on-every-post.md) and
    [ADR-035](adr/035-a-proposed-post-waits-as-the-persons-draft.md).
