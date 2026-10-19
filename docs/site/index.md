---
title: An MCP server for Mattermost
hide:
  - navigation
  - toc
---

<div class="mm-hero" markdown>

<div markdown>

# Your Mattermost, within your agent's reach

<p class="mm-hero__lead">mm-mcp is an MCP server for Mattermost. It lets an AI
agent read and search your Mattermost, and, when you allow it, post and reply,
as you or as a bot. Each post states how far it goes, and mm-mcp checks that
before it is sent.</p>

[Install mm-mcp](installation.md){ .md-button .md-button--primary }
[Connect your account](login.md){ .md-button }

</div>

<div class="mm-chat" role="img" aria-label="A conversation: you ask your agent what you missed in a channel, it reads the unread posts through mm-mcp and sums them up, and before it replies for you, your AI app asks whether to allow the post, which states where it goes and how many people it reaches.">
<div class="mm-chat__bar"><span class="mm-chat__dot"></span>Your agent, with mm-mcp</div>
<div class="mm-chat__body">
<div class="mm-msg mm-msg--you">What did I miss in ~release-planning?</div>
<div class="mm-tool">read_unread ~release-planning</div>
<div class="mm-msg mm-msg--agent">Two threads since Friday. The release moved to Thursday, and Sam asks whether the 2.4 notes are ready.</div>
<div class="mm-msg mm-msg--you">Tell Sam they are.</div>
<div class="mm-tool">create_post …/channels/release-planning, reaches: 14</div>
<div class="mm-msg mm-msg--agent mm-ask">Allow create_post in ~release-planning, reaching 14 people?<br><em>“The 2.4 notes are ready, Sam.”</em>
<div class="mm-ask__buttons"><span>Allow</span><span>Deny</span></div>
</div>
</div>
</div>

</div>

## What it is for

<div class="grid cards" markdown>

-   :material-text-search:{ .lg .middle } **Reading and searching**

    ---

    Channels, threads, unread messages and mentions, and people and channels
    looked up by the names you use.

-   :material-send-check-outline:{ .lg .middle } **Posting and replying**

    ---

    Posts, replies, edits, reactions and pins, joining and creating channels,
    and your status, once you allow writes, each approved by your AI app and
    checked by mm-mcp before it is sent.

-   :material-bookmark-outline:{ .lg .middle } **Your own things**

    ---

    Following threads, saved posts, reminders, drafts, the typing indicator and
    marking a channel read, which change only what is yours.

-   :material-paperclip:{ .lg .middle } **Files and attachments**

    ---

    Read as text or images the model can use, uploaded, and saved to your
    machine.

</div>

**Views in the chat**: in a client that renders MCP Apps, a thread, what is
unread for you, or a post your agent made appears as Mattermost lays it out,
and a draft that appears before it is posted is planned
([ADR-022](adr/022-mcp-apps-views-on-show-and-on-every-post.md)).

## How it behaves

<div class="grid cards" markdown>

-   :material-shield-check-outline:{ .lg .middle } **Read-only unless you say otherwise**

    ---

    The tools that post or change anything are not even offered until you
    allow writes. Then your AI app's own approval of each call decides, and
    mm-mcp checks what each change states before it goes out: how many people a
    post reaches and notifies, the text an edit replaces, what a deletion takes
    away, and where it goes. Each post is marked as written with AI, unless you
    turn that off ([Configuration](configuration.md)).

-   :material-account-key-outline:{ .lg .middle } **One account, yours or a bot's**

    ---

    It acts as whoever owns the token it is given: a personal access token, a
    bot's token, or your own login through [`mm-mcp login`](login.md), single
    sign-on included. It can do what that account can, and no more.

-   :material-laptop:{ .lg .middle } **On your own machine**

    ---

    Your AI app starts mm-mcp on your computer and talks to it directly.
    Nothing is hosted, and mm-mcp talks to nothing but your Mattermost
    ([Security](security.md)).

-   :material-test-tube:{ .lg .middle } **Tested against a real Mattermost**

    ---

    Every tool is exercised against Team Edition, on the current Extended
    Support Release and the newest release
    ([Supported releases](mattermost-releases.md)).

</div>

## What it does well

- **Nothing to install on the server.** mm-mcp talks to Mattermost's API as its
  own apps do, so it needs no plugin, and works with the free Team Edition too.
- **States what each post does.** A post names where it goes by its address,
  and how many people it reaches and an `@channel` notifies; an edit writes out
  the whole text it replaces. mm-mcp checks each against Mattermost before it
  acts, and a refusal says what it found.
- **Answers shaped for an agent.** A post comes with its author, channel, team,
  files and reactions; channels, teams and people are found by the names you use,
  and a name that matches nothing comes back with the closest ones.
- **Reads as Mattermost does.** Unread posts from where you stopped, without
  marking them read; mentions as your notification settings define them; threads
  as its threads view lists them.
- **Reads your files.** Word, PowerPoint and Excel as their text, archives as a
  listing, images upright and scaled, and text in windows of numbered lines.
- **Logs in where tokens are off.** Through your own browser, single sign-on and
  second factor included ([Logging in](login.md)).

Why it works the way it does, one decision at a time, is in the
[decision records](adr/index.md).

mm-mcp is an independent project. It is not affiliated with, endorsed by or
supported by Mattermost, Inc.; Mattermost is a trademark of Mattermost, Inc.
