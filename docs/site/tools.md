---
hide:
  - navigation
---

# Tools

<p class="mm-lead">Everything your agent can do in Mattermost with mm-mcp. Ask in
your own words; it picks the tool. Writing is off until you allow it.</p>

<div class="mm-stats">
<a class="mm-stat" href="#every-tool"><strong>48</strong> tools</a>
<span class="mm-stat mm-stat--read"><strong>26</strong> only read</span>
<span class="mm-stat mm-stat--checked"><strong>14</strong> checked writes</span>
<span class="mm-stat mm-stat--yours"><strong>7</strong> yours alone</span>
<span class="mm-stat mm-stat--disk"><strong>1</strong> saves to your disk</span>
<a class="mm-stat mm-stat--view" href="../views/"><strong>5</strong> show you a view</a>
</div>

<div class="grid cards mm-jobs" markdown>

-   :material-inbox-arrow-down:{ .lg .middle } **Catch up**

    ---

    Pick up where you left off, in every channel, without marking anything read.

    “What did I miss since Friday?”
    { .mm-example }

    [`read_unread`](#read_unread){ .mm-chip } [`get_user_channels`](#get_user_channels){ .mm-chip } [`list_mentions`](#list_mentions){ .mm-chip } [`list_threads`](#list_threads){ .mm-chip }

-   :material-text-search:{ .lg .middle } **Find it again**

    ---

    Messages, files, people and channels, searched the way you would say it: by
    words, by who, where and when.

    “What did we decide about the release date?”
    { .mm-example }

    [`search_posts`](#search_posts){ .mm-chip } [`search_files`](#search_files){ .mm-chip } [`search_users`](#search_users){ .mm-chip } [`search_channels`](#search_channels){ .mm-chip }

-   :material-forum-outline:{ .lg .middle } **Read the whole story**

    ---

    A channel page by page, a thread from its first post, and what was pinned or
    saved.

    “Summarise the thread about last night's outage.”
    { .mm-example }

    [`read_channel`](#read_channel){ .mm-chip } [`read_post`](#read_post){ .mm-chip } [`list_pinned_posts`](#list_pinned_posts){ .mm-chip } [`list_saved`](#list_saved){ .mm-chip }

-   :material-eye-outline:{ .lg .middle } **See it as Mattermost shows it**

    ---

    In an AI app that shows MCP Apps views, a thread, or what is waiting for
    you, appears in the chat as Mattermost lays it out: names and pictures,
    Markdown, reactions and images, and what is unread where.

    “Show me the thread about last night's outage.”
    { .mm-example }

    [`show`](#show){ .mm-chip } [`view_action`](#view_action){ .mm-chip } [`view_change`](#view_change){ .mm-chip .mm-chip--yours }

-   :material-file-document-outline:{ .lg .middle } **Open the attachments**

    ---

    Word, Excel and PowerPoint as their text, PDFs, images and archives, read for
    the model, or saved to your disk.

    “What's in the spreadsheet Sam shared?”
    { .mm-example }

    [`read_file`](#read_file){ .mm-chip } [`save_file`](#save_file){ .mm-chip .mm-chip--disk }

-   :material-send-check-outline:{ .lg .middle } **Post, reply and react**

    ---

    Messages, replies, direct and group messages, edits and reactions, each
    approved by your AI app and checked by mm-mcp before it goes out.

    “Tell Sam in the thread that the notes are ready.”
    { .mm-example }

    [`create_post`](#create_post){ .mm-chip .mm-chip--checked } [`dm`](#dm){ .mm-chip .mm-chip--checked } [`group_message`](#group_message){ .mm-chip .mm-chip--checked } [`update_post`](#update_post){ .mm-chip .mm-chip--checked } [`delete_post`](#delete_post){ .mm-chip .mm-chip--checked } [`add_reaction`](#add_reaction){ .mm-chip .mm-chip--checked } [`remove_reaction`](#remove_reaction){ .mm-chip .mm-chip--checked } [`pin_post`](#pin_post){ .mm-chip .mm-chip--checked }

-   :material-account-group-outline:{ .lg .middle } **Channels and people**

    ---

    Who is who and who is around, which channels exist and who is in them; join,
    leave, create, add people, and set your status.

    “Is Sam online, and who else is in ~release-planning?”
    { .mm-example }

    [`get_me`](#get_me){ .mm-chip } [`get_users`](#get_users){ .mm-chip } [`get_status`](#get_status){ .mm-chip } [`get_user_teams`](#get_user_teams){ .mm-chip } [`get_team_info`](#get_team_info){ .mm-chip } [`get_channel_info`](#get_channel_info){ .mm-chip } [`list_team_channels`](#list_team_channels){ .mm-chip } [`list_archived_channels`](#list_archived_channels){ .mm-chip } [`get_channel_stats`](#get_channel_stats){ .mm-chip } [`join_channel`](#join_channel){ .mm-chip .mm-chip--checked } [`leave_channel`](#leave_channel){ .mm-chip .mm-chip--checked } [`add_channel_members`](#add_channel_members){ .mm-chip .mm-chip--checked } [`create_channel`](#create_channel){ .mm-chip .mm-chip--checked } [`set_status`](#set_status){ .mm-chip .mm-chip--checked }

-   :material-bookmark-outline:{ .lg .middle } **Your own things**

    ---

    A post proposed as your draft, to send yourself, reminders, saved posts and
    followed threads. Nobody else sees them; deleting a draft, which loses your
    words, is checked.

    “Draft a reply I can send myself, and remind me at nine tomorrow.”
    { .mm-example }

    [`propose_post`](#propose_post){ .mm-chip .mm-chip--yours } [`list_drafts`](#list_drafts){ .mm-chip } [`delete_draft`](#delete_draft){ .mm-chip .mm-chip--checked } [`set_post_reminder`](#set_post_reminder){ .mm-chip .mm-chip--yours } [`save_post`](#save_post){ .mm-chip .mm-chip--yours } [`follow_thread`](#follow_thread){ .mm-chip .mm-chip--yours } [`mark_channel_read`](#mark_channel_read){ .mm-chip .mm-chip--yours } [`typing`](#typing){ .mm-chip .mm-chip--yours }

-   :material-stethoscope:{ .lg .middle } **Check the setup**

    ---

    When something fails: the connection, the login, and who decides on a
    write.

    “Why can't you post?”
    { .mm-example }

    [`diagnose`](#diagnose){ .mm-chip }

</div>

## Every tool

Grouped as above. A badge says when a tool does more than read:
<span class="mm-badge mm-badge--checked">checked</span> for a change others see,
or words of yours it deletes, which the call states and mm-mcp checks
([below](#how-a-write-is-checked));
<span class="mm-badge mm-badge--yours">yours alone</span> for what only you see
or is gone in seconds; and <span class="mm-badge mm-badge--disk">your disk</span>.
A tool marked <span class="mm-badge mm-badge--view">view</span> also shows you
what it did in the chat, in an AI app that shows views; the badge opens it,
live, on [Views](views.md).
The tools that write are offered only once you allow writes, with
`MM_MCP_ALLOW_WRITES` ([Configuration](configuration.md)).

### Catch up

`read_unread`: Read unread posts { #read_unread }
:   Catches up on a channel from where you stopped reading: a few posts you have
    read, then the ones you have not, oldest first, with the first unread one
    named. A channel you never opened is all unread, and reads newest first,
    the pages after going further back. It marks nothing read: what you have
    read is yours to record.

`get_user_channels`: List the user's channels { #get_user_channels }
:   The channels you belong to, direct and group messages included, most
    recently active first, each with its team and how many messages and
    mentions are unread. `team_id` keeps one team's, `unread_only` those with
    something unread.

`list_mentions`: List mentions { #list_mentions }
:   The posts that mention you, most recent first, as Mattermost's Recent
    Mentions finds them: by `@username`, and by your first name, the other
    words and `@channel`, `@all` and `@here` when your notification settings
    say those mention you. Only a post that mentions you as Mattermost would
    notify you is kept, and none of your own. The answer names the words it
    looked for. `from`, `in`, `before`, `after` and `on` narrow it, and like
    `search_posts` it reaches the 100 most recent matches at most. The
    database search Team Edition uses skips "all" and "here" as too common to
    index, so `@all` and `@here` are found only on a server that searches with
    Elasticsearch, as they are in Mattermost's own Recent Mentions.

`list_threads`: List threads { #list_threads }
:   The threads you follow, as Mattermost's threads view lists them, most
    recently replied to first: the post that started each, who took part, and
    how many replies and mentions you have not read.

### Find it again

`search_posts`: Search posts { #search_posts }
:   Messages you can read, across every team or one, by words, and by who wrote
    them, where and when through `from`, `in`, `before`, `after` and `on`, so no
    search syntax is needed. `@username` finds where someone was mentioned.
    20 posts a page by default, at most 100. Mattermost's search finds the 100
    most recent matches at most, and the answer says `capped` when it reached
    them: `before` reaches older ones. `in` takes a channel you belong to, the
    only ones Mattermost searches, and `from` a username, refused with the
    closest when nobody has it. The days `before`, `after` and `on` name are
    your own, in the timezone you set in Mattermost, as in its search box.
    The database search Team Edition uses looks in a post's message alone, so
    a post whose words are only in its attachments or blocks, as an alert's
    often are, is not found; Elasticsearch finds them. The description says
    so, and points the model to `read_channel` with `since` for a channel of
    alerts.

`search_files`: Search files { #search_files }
:   Files attached to posts you can read, by name, by type with `ext:pdf`, and by
    `from`, `in`, `before`, `after` and `on`, each with the post and channel it
    is in and their links, and the file's address as a
    [resource](#files-as-resources). Like `search_posts`, it finds the 100 most
    recent matches at most.

`search_users`: Search users { #search_users }
:   Users whose username, name, nickname or email address contains a term,
    optionally only within a team or a channel. Deactivated users are left out.
    Mattermost finds at most 1000 for one term.

`search_channels`: Search channels { #search_channels }
:   Channels by part of their name: your own, and public ones you have not
    joined. Mattermost finds at most 50 public channels in a team for one term.

### Read the whole story

`read_channel`: Read channel { #read_channel }
:   A channel's messages, 30 a page by default and at most 200, oldest first
    within a page: the newest, and the pages after go further back. `before`
    starts back from a post; `after` and `since`, from a post or a time, read
    forward. `since` takes a time with an offset, or a time or a day in your
    own timezone. `collapse_threads` leaves the replies out and shows each
    thread by the post that started it, as Mattermost shows a channel with
    collapsed reply threads.

    A page here or of `read_unread` can hold a few more posts than its limit:
    posts written in the same millisecond stay on one page, since Mattermost
    reads on from a post by its time alone, and a page split inside a
    millisecond would lose posts.

`read_post`: Read post { #read_post }
:   A post and the thread it is in, oldest first, 100 posts a page by default.
    Give any post in the thread; `include_thread` false reads the post alone.

`list_pinned_posts`: List pinned posts { #list_pinned_posts }
:   The posts pinned to a channel.

`list_saved`: List saved posts { #list_saved }
:   The posts you saved to come back to.

### See it as Mattermost shows it

`show`: Show [view](views.md#see-a-thread){ .mm-view-badge } { #show }
:   Puts what you asked to see in front of you in the chat, laid out as
    Mattermost lays it out.

    - Kind `thread` shows a thread: who wrote each post, with their profile
      picture, when, the Markdown drawn, `@mentions` and `~channels`, the
      cards a webhook or bot posted beside their colored bars, the blocks
      an integration posted with their columns and folded sections, the post a
      link quotes and the card of a page it leads to, reactions with
      who reacted, the Important and Urgent labels and how many acknowledged
      a post that asks for it, images you can open whole and file cards, the
      edited and pinned marks, a burn-on-read post you have not opened as
      a placeholder that opens it in Mattermost, a call as the Calls plugin's
      card, and files an access policy withholds as Mattermost's placeholder.
      It shows the first 30 posts, says how
      many more the thread has, and loads more as you ask.
    - Kind `unreads` shows what is waiting for you: up to 20 channels with
      something unread, grouped by team, with their unread and mention
      counts; up to 10 posts that mention you; and up to 10 threads you follow
      with unread replies. It says when Mattermost has more. A channel's
      unread messages or a thread open in place, or in Mattermost where your
      app does not let the view load them, and once you allow writes, a
      channel can be marked read from the view.

    Links open in Mattermost. Your agent calls it when you ask to see
    something, after it found it, or ask what is waiting for you. It answers
    with what `read_post`, or `get_user_channels`, `list_mentions` and
    `list_threads`, answer, so the chat holds nothing the view adds. In an app
    that shows no views it shows nothing, and says so, and your agent answers
    in words ([Views](#views)).

`view_action`: Act in a view { #view_action }
:   What you do in a view that changes nothing, such as loading more of a
    thread or opening a channel's unread messages in place, goes through this
    tool. Only a view calls it: an app that shows views keeps it from your
    agent, and mm-mcp offers it only to an app that says it shows views.

`view_change`: Change in a view { #view_change .mm-yours }
:   What you do in a view that changes something goes through this tool:
    marking a channel read from the unread view, as `mark_channel_read` does,
    and posting, changing or discarding a proposal
    ([`propose_post`](#propose_post)). Only a view calls it, on your press: an
    app that shows views keeps it from your agent, and mm-mcp offers it only to
    an app that says it shows views, once you allow writes.

### Open the attachments

`read_file`: Read file { #read_file }
:   A file attached to a post, as content the model can read: text in windows of
    numbered lines, chosen with `start_line` and `line_count`; Word, PowerPoint
    and Excel files as their text; zip and tar archives as a listing; images as
    images, turned upright and scaled down when large; small audio, video and
    PDF files as themselves, for a client that can read them. Anything else is
    described by its type and size, with the post's link. Files over 64 MiB are
    described without being read.

`save_file`: Save file { #save_file .mm-disk }
:   Saves a file attached to a post into your download directory, set with
    `MM_MCP_DOWNLOAD_DIR` and your Downloads directory by default, under its own
    name, and answers with the path. An existing file is never overwritten: a
    number is added to the name instead, and saving the same file again
    answers with the copy already saved. The file is marked as downloaded from
    the internet, as a browser marks it: on Windows, SmartScreen checks a
    program and Office opens a document in Protected View; on macOS,
    Gatekeeper checks it. Files up to 100 MiB. Offered only when mm-mcp runs on
    your machine, whether writes are allowed or not
    ([below](#on-your-machine)).

### Post, reply and react

Your MCP client's approval of each call decides, and mm-mcp checks what each
states before it acts ([below](#how-a-write-is-checked)).

`create_post`: Create post [view](views.md#see-what-your-agent-posted){ .mm-view-badge } { #create_post .mm-checked }
:   Posts a message in a channel, or with `root_id` replies in the thread of any
    post in it. Where mm-mcp asks, its question shows the message as it will be
    sent, the channel and, for a reply, the start of the thread. Posting takes
    down the typing indicator `typing` put up there. Returns the post, which an
    app that shows MCP Apps views shows in the chat as posted, under where it
    went, as `dm` and `group_message` do ([Views](#views)).

    Every call states how far the message goes
    ([below](#how-far-a-message-goes)): `reaches`, how many members the
    channel has, and `notifies` where the message alerts a whole channel or
    user group as Mattermost would ask about first.

    `files` attaches up to ten files: text the model writes, given a `name` and
    `content`, or, when the server runs on your machine over stdio, a file by
    its full `path`. Where mm-mcp asks, its question lists each file with its
    size, type and the path it was read from, the box you tick names every file
    read from your disk by its path, the files are uploaded only once you
    accept, and a file changed after you were asked is refused. A file where
    credentials are kept is never attached
    ([Security](security.md#credentials)); attach such a file in Mattermost
    yourself if you mean to. `dm` and `group_message` take `files` too.

`dm`: Send direct message [view](views.md#see-what-your-agent-posted){ .mm-view-badge } { #dm .mm-checked }
:   Sends a direct message to one person, by username or email address, or a
    note to yourself by your own username. The username is required, so
    whatever approves the call sees who it goes to. This is how to message
    people: "message Alice and Bob" is two direct messages.

`group_message`: Send group message [view](views.md#see-what-your-agent-posted){ .mm-view-badge } { #group_message .mm-checked }
:   Sends one message to two to seven people together, in the group conversation
    Mattermost keeps for exactly them. Only when you ask for a group
    conversation.

`update_post`: Update post { #update_post .mm-checked }
:   Replaces the text of one of your own posts. Where mm-mcp asks, its question
    shows the current text and its replacement. An edit notifies nobody, so
    its mentions are not checked, only its length. The call states `reaches`,
    how many members the post's channel has, since an edit reaches everyone the
    post does; `old_message`, the post's current text in full; and, for a post
    with reactions or replies, `responses`, their number
    ([below](#what-an-edit-changes)). Another person's post is refused, even
    with an administrator's credential.

`delete_post`: Delete post { #delete_post .mm-checked }
:   Deletes one of your own posts. Deleting the post that starts a thread
    deletes its replies, other people's included. The call states `deletes`, how
    many posts go: 1 for a reply, or the post and its replies
    ([below](#what-a-write-takes-away)).

`add_reaction`: Add reaction { #add_reaction .mm-checked }
:   Reacts to a post with an emoji, by name, such as `thumbsup`, or as itself,
    such as 👍. An unknown name is refused with the closest ones. A reaction
    joins one others gave under another name of the same emoji, so
    `thumbsup` beside a `+1` is the one 👍. Reacting again with the same
    emoji changes nothing.

`remove_reaction`: Remove reaction { #remove_reaction .mm-checked }
:   Takes back one of your reactions.

`pin_post`: Pin post { #pin_post .mm-checked }
:   Pins a post to its channel for everyone, or unpins it with `pinned` false.

### Channels and people

`get_me`: Who am I { #get_me }
:   The Mattermost user the server acts as: id, username, name, nickname,
    position, roles, the timezone set in Mattermost, and whether it is a bot.
    Call it to check the connection and whose access the other tools use.

`get_users`: Get users { #get_users }
:   People by username, user id or email address, any mix of them in one call,
    with whether each is a bot or deactivated. An email address finds someone
    only when the server shows you addresses. A name nobody has is listed
    with the closest usernames beside the people found, and refused only when
    nobody is found.

`get_status`: Get status { #get_status }
:   Whether people are around: online, away, do not disturb or offline, when
    they were last active, and the status message they set.

`get_user_teams`: List the user's teams { #get_user_teams }
:   The teams you belong to.

`get_team_info`: Get team { #get_team_info }
:   A team by id, name or link. An open team you are not in is found by the
    exact name in its link.

`get_channel_info`: Get channel { #get_channel_info }
:   A channel by id, name or link, among your own channels and the public
    channels of your teams, saying whether you belong to it and whether it is
    archived.

`list_team_channels`: List a team's channels { #list_team_channels }
:   A team's public channels, by name.

`list_archived_channels`: List archived channels { #list_archived_channels }
:   A team's archived channels. An archived channel can be read but not posted
    in.

`get_channel_stats`: Get channel stats { #get_channel_stats }
:   How many people belong to a channel, how many are guests, and how many posts
    are pinned and files shared in it.

`join_channel`: Join channel { #join_channel .mm-checked }
:   Joins a public channel of one of your teams. Everyone in it sees that you
    joined. A private channel is joined only by being added.

`leave_channel`: Leave channel { #leave_channel .mm-checked }
:   Leaves a channel you belong to. Everyone in it sees that you left, and
    only being added brings you back to a private one, which mm-mcp's question
    says where it asks.

`add_channel_members`: Add channel members { #add_channel_members .mm-checked }
:   Adds up to twenty people, by username, user id or email address, to a
    channel you belong to. Each is notified, and where mm-mcp asks, its question
    names each.

`create_channel`: Create channel { #create_channel .mm-checked }
:   Creates a public or private channel in one of your teams, with you in it,
    the name in its link made from its display name as Mattermost's own app
    makes it, and its purpose and header when given.

`set_status`: Set status { #set_status .mm-checked }
:   Sets your status as everyone sees it beside your name: online, away, do not
    disturb until a time, or offline, and a status message with an emoji, until
    a time or until changed; or clears the message. A time without an offset
    is read in your timezone.

### Your own things

`propose_post`: Propose post [view](views.md#check-and-send-a-post-your-agent-drafts){ .mm-view-badge } { #propose_post .mm-yours }
:   Proposes a post for you to send yourself. The message waits as your draft,
    in a channel, a thread, or a direct or group message you already have, in
    your message box in every Mattermost app, and the answer links where it
    waits. Nothing is sent. The message is checked as a post's is, and its notes
    say what its mentions will do. One that starts with `/` is refused, since
    Mattermost runs it as a slash command when you send it. Proposing again replaces mm-mcp's own
    proposal while you have not changed it; a draft you wrote or typed in is
    never replaced ([below](#what-a-write-takes-away)). A conversation that
    does not exist yet is not started, since starting one shows it to the other
    people at once, and a proposal is refused when your drafts do not sync,
    since you would never see it. What you send from Mattermost goes out as
    yours, without the mark of a post written with AI. Whatever your app
    approves, it is a way to post only what you send yourself.

    In an app that shows views, the proposal appears in the chat as it will be
    posted, where it waits, and what its mentions do, with Post, Edit and
    Discard ([Views](#views)). Each acts on the text the view shows, and only
    while your draft still holds it as the view put it there: a draft you typed
    on in Mattermost is yours to send from there, and a newer proposal is drawn
    again as it is now. Post marks the post as written with AI, and asks
    first, in Mattermost's own words, where Mattermost would ask before an
    `@channel` or `@here` reaches more than five people. A link shows where it
    leads, a character you cannot see or that turns the text's direction is
    flagged, and the text as written is a click away. Where a draft of yours is
    in the way, the view shows yours beside the proposal, or the proposal as a
    revision of yours when your agent says so with `revises_draft`, and nothing
    is written until you press Replace mine, or Apply, or Keep mine.
    `MM_MCP_POST_FROM_VIEWS=false` takes Post out of the view
    ([Configuration](configuration.md)).

`list_drafts`: List drafts { #list_drafts }
:   Your drafts, in channels and threads, with where each is, by link and
    address, and its `length`, which `delete_draft` states.

`delete_draft`: Delete draft { #delete_draft .mm-checked }
:   Deletes your draft in a channel or thread. Nobody else sees a draft, but it
    may hold words you have not sent, so the call states `discards`, how many
    characters the draft holds, as `list_drafts` gives its length
    ([below](#what-a-write-takes-away)), and where mm-mcp asks, its question
    shows the draft and where it is.

`set_post_reminder`: Set post reminder { #set_post_reminder .mm-yours }
:   Has Mattermost remind you of a post at a time, as its "Remind me" does. A
    time without an offset is read in the timezone you set in Mattermost.

`save_post`: Save post { #save_post .mm-yours }
:   Saves a post among your saved posts, or removes it with `saved` false.

`follow_thread`: Follow thread { #follow_thread .mm-yours }
:   Follows a thread, so its replies notify you and it appears in
    `list_threads`, or stops following it with `following` false.

`mark_channel_read`: Mark channel read { #mark_channel_read .mm-yours }
:   Marks a channel read, as opening it in Mattermost does, clearing its unread
    count and mentions, only when you ask for it. Reading a channel for you
    never marks it read.

`typing`: Show typing { #typing .mm-yours }
:   Shows you typing in a channel or thread while a message is written, kept up
    until `create_post` posts there, `stop` is sent, or a minute passes. It
    is optional, for a long message that takes a while to write.

### Check the setup

`diagnose`: Diagnose mm-mcp { #diagnose }
:   The checks `mm-mcp doctor` makes, from inside the running server, so with
    the configuration your MCP client actually started it with: the address,
    which credential it acts with, a login `mm-mcp login` stored and whether
    `MM_TOKEN` hides it, the proxy, whether the server answers as a supported
    Mattermost, whose credential it is and whether that user belongs to a team.
    It adds what only the client shows: its name and protocol version, who
    decides on a write, and, where mm-mcp is set to ask, whether the client can
    show its question. With `ask_test_question`, it shows you a
    test question and reports how it was answered: an answer within a few
    seconds came from the client itself, which then answers every question
    before a write without showing it. Each check says what it found, and a
    failed one what to do. It never returns the credential
    ([Configuration](configuration.md#command-line)).

## Good to know

### Works the same everywhere

- **Names and links work where ids do, for a tool that reads.** Every
  `channel_id` and `team_id` of a tool that reads, or that changes only what is
  yours, takes a name as well as an id, a channel's with `~` or `#` before it or not.
  Every id argument takes the place's address, and the link Mattermost shows
  you: a channel's, a direct or group message's, a team's, a post's, Copy
  Link's or the one a thread opens at, and a file's, or its
  [resource](#files-as-resources) address. A name of a
  channel, a team or a person matches in any case, whole or in part; an
  ambiguous one is refused with every candidate, and an unknown one with the
  closest names. A link is never fetched, and one to another server is refused
  ([ADR-030](adr/030-names-are-matched-leniently-and-a-refusal-names-the-next-step.md)).
- **A write others see takes an address.** The tools that post or change what
  others see take a channel, a post or a team by its address alone, matched
  exactly: `mattermost://<server>/<team>/channels/<channel>`, a direct
  message's `mattermost://<server>/<team>/messages/@<username>`, a post's as
  its channel's address followed by its id, and a team's as
  `mattermost://<server>/<team>`. Your MCP client's approval of the call, or
  its AI reviewer, sees the tool's arguments and nothing else, so the address
  shows where the write goes by name. An id, a name or a link is refused with
  where to read the address, and so is the old address of a renamed channel.
- **Every place comes with its link and its address.** Each post, channel,
  team and draft an answer describes carries its address, and the link it opens
  at, and so do the post and channel of each file `search_files` finds, made
  from names: a channel's in its
  team, a direct or group message's in your first team, as
  `/<team>/messages/@<username>`, and a post's in its channel's team, as
  Copy Link gives it. Asked where something was said, the agent links the post,
  not the conversation. Beside each link is the name Mattermost shows: a
  channel's display name, and a direct or group message named after the other
  people in it.
- **Every list pages the same way.** Give `limit`, and pass the `next_cursor` an
  answer ends with back as `cursor`, with the same other arguments, for the next
  page, until an answer has none
  ([ADR-032](adr/032-every-list-pages-by-an-opaque-cursor.md)).
- **Every post comes back in one shape:** its link and address, its author's
  username and name, its channel by name, link and address and its team by
  name, its files with their ids and resource addresses, reactions with who
  reacted, `responses`, how many reactions and replies it has, whether it is
  pinned or marked as written with AI, its thread, and when it was edited. An
  answer whose posts are all in one channel names the channel and team once.
- **A post from a webhook or a bot carries its cards.** Integrations such as
  Alertmanager, Grafana, Jira or a CI server post their content as message
  attachments, often with no message beside them. Each post gives them as
  `message_attachments`, as Mattermost shows them: the pretext, author and
  its link, title and its link, text, fields, footer and fallback, the color
  the integration chose for the bar beside them, and the addresses of their
  pictures, `image_url`, such as a chart Grafana drew, and `thumb_url`. Their
  buttons and menus are left out: pressing one is a change, and it calls the
  integration. The color is kept where Mattermost draws it: `good`,
  `warning`, `danger` and hex colors everywhere, and from 11.11 on, where
  Mattermost draws attachments as blocks, any CSS color, such as `red` or
  `rgb(220, 0, 0)`, as the integration wrote it. So is a container's.
- **A post from an integration carries its blocks.** From Mattermost 11.11 on,
  integrations can post their content as blocks, in Mattermost's own format,
  Slack's Block Kit or Microsoft's Adaptive Cards, often with no message
  beside them. Where your server shows them, each post gives them as `blocks`,
  in one shape whichever format they came in, as Mattermost draws them: text,
  images by address, dividers, containers with the color of their bar,
  collapsible sections and columns, as an outline in which a block holds the
  deeper blocks after it. Mattermost draws a post's blocks in place of its
  message attachments, and so does mm-mcp. Buttons and menus are left out,
  as they are of message attachments. A server on an older release does not
  show blocks, and neither does mm-mcp
  ([Supported releases](mattermost-releases.md#differences-in-what-a-release-shows)).
- **A post carries what its first link leads to, as Mattermost shows it below
  the message.** A link to another post gives `quoted_post`: its author,
  channel, time, link, message, label and files, so "is this still true?"
  followed by a link reads whole. Mattermost leaves the quote out for you
  when you cannot read the post it quotes. A link to a web page gives
  `link_preview` with the page's title, description and site, and a link to a
  picture `link_preview` with `image` and its size. As in Mattermost, a post
  with message attachments shows no preview, and the page's text is its
  owner's.
- **A post from an integration says so, under the name Mattermost shows.**
  An integration posts under an account, a person's or a bot's, through an
  incoming webhook or as a slash command's answer. Such a post has
  `from_integration`, and a bot's post has `author_is_bot`: Mattermost tags
  both BOT. Where your server lets integrations choose their name, `shown_as`
  is the name Mattermost shows in place of the account's, and `author` still
  names the account. Anyone who can call the integration can choose any name,
  so the account is the one to trust.
  [`show`](#show) draws such a post as Mattermost does: the name, the BOT tag,
  and no run with the posts around it.
- **A post marked Important or Urgent says so.** Its `priority` is
  `important` or `urgent`, the label Mattermost shows beside its author's
  name, and `persistent_notifications` says Mattermost keeps notifying the
  people it mentions until they read it. On a licensed server a post can ask
  everyone to acknowledge it: it has `requested_ack`, `acknowledgements` says
  who did and when, and `acknowledged` that you did. Each is there only where
  your server shows it, as Mattermost's settings for message priority and
  acknowledgements say.
- **A burn-on-read post stays concealed.** On a server with an Enterprise
  Advanced license, someone can send a post of `type` `burn_on_read`, which
  Mattermost conceals from you until you open it there; a while after, it is
  gone for you. Until you open it, the post has `concealed` and nothing of
  what it holds. No tool opens one: your agent can tell you it waits, and its
  `url` takes you to it. `expire_at` is when it is gone for you: once opened,
  when its timer runs out, and until then, the latest it is kept.
- **A call carries the call.** The Calls plugin, on by default, starts a call
  with a post of `type` `custom_calls`, and draws its own card in place of the
  message. Such a post has `call`: its title, `started_at`, and once it ended,
  `ended_at` and the usernames of its `participants`. The attachment the
  plugin adds for clients without it, which repeats the message, is left out.
  Joining a call is for you to do in Mattermost.
- **Other posts read as Mattermost shows them.** A `/me` post has `type`
  `me`: Mattermost shows its message without the asterisks around it. A
  reminder the system bot sends you links to its post whole, as Mattermost
  does, also on a server without a site URL, whose reminders hold only a
  path. On a licensed server, `withheld_files` says how many files an access
  policy keeps from you, where Mattermost shows a placeholder.
- **A list cuts a long post short.** A list shows at most 4,000 characters of
  a post: its message first, then the text of its message attachments or
  blocks. What is cut says its whole length, `message_length`,
  `message_attachments_length` or `blocks_length`; `read_post` reads it whole.
- **Times are your own.** A time in an answer is in the timezone you set in
  Mattermost, with its offset, such as `2026-10-09T09:30:00+02:00`, and a time
  you give without an offset is read in it too. With no timezone set, UTC.

### Files as resources

Every file attached to a post is also an MCP resource, at
`mattermost://<your server>/files/<file id>`, from a template mm-mcp lists
([ADR-029](adr/029-files-reach-the-model-as-content-and-the-disk-only-locally.md)).
A client can read one when it needs it, and some let you attach one to the
conversation or save it. It reads as [`read_file`](#read_file) reads the file,
but whole: text, a Word, PowerPoint or Excel file's text and an archive's
listing as text, an image as the image `read_file` returns, and anything else
as itself, in one message of up to 10 MiB, a binary file counted as its
base64: MCP sends a resource whole, and clients drop a larger message.
`MM_MCP_RESOURCE_MAX_MB` changes that for a client that takes more
([Configuration](configuration.md)). A file larger than your Mattermost now
takes in one, its Maximum File Size, is not downloaded.
Past a limit, a text is read with `read_file`, in windows, and a file saved
with `save_file`.
Each post's files, and each file `search_files` finds, give the address, the
answer links each file as a resource, and every `file_id` takes the address. An
image also gives its size and its preview's address, at
`mattermost://<your server>/files/<file id>/preview`: the smaller image its post
shows, cheaper to look at than the whole image.

The pictures a post shows with it are resources too: a person's profile picture,
at `mattermost://<your server>/users/<user id>/image`, the one they set or the
one Mattermost draws from their initials, and a custom emoji's, at
`mattermost://<your server>/emoji/<name>`. Each reads as a picture, PNG, JPEG,
GIF or WebP and never SVG, and one read again within a few minutes comes from
what mm-mcp kept rather than from Mattermost. A
client that completes resource addresses offers, as you type part of a file's
name, the ids of the files that match. Threads and conversations are not
resources: a conversation has no end to read it whole to.

### Views

In an AI app that shows MCP Apps views, [`show`](#show) puts a thread, or what
is unread for you, in the chat as Mattermost lays it out, every post your
agent makes shows itself there as posted, and a post it proposes with
[`propose_post`](#propose_post) waits there for you to post, change or discard
([ADR-022](adr/022-mcp-apps-views-on-show-and-on-every-post.md)). The view is a
page in mm-mcp itself, which reaches no network: the app reads what it shows
from mm-mcp, and the pictures as [resources](#files-as-resources), with mm-mcp's
credential. Where the app reads no pictures, people are their initials and a
custom emoji its name. The page takes the app's light or dark theme and its
language, where mm-mcp has words for it, and English otherwise. Goose's desktop
app shows views, and so does VS Code with its `chat.mcp.apps.enabled` setting
on; `diagnose` says whether yours does. [Views](views.md) shows each of them,
live.

### Page sizes

What a page holds when `limit` is not given, and the most it can hold.

| Tool | Default | At most |
|---|---|---|
| [`read_channel`](#read_channel), [`read_unread`](#read_unread) | 30 | 200 |
| [`read_post`](#read_post) | 100 | 200 |
| [`list_pinned_posts`](#list_pinned_posts) | 30 | 100 |
| [`search_posts`](#search_posts), [`list_mentions`](#list_mentions), [`search_files`](#search_files) | 20 | 100 |
| [`list_saved`](#list_saved), [`list_threads`](#list_threads), [`list_drafts`](#list_drafts), [`search_users`](#search_users) | 20 | 100 |
| [`get_user_teams`](#get_user_teams), [`get_user_channels`](#get_user_channels), [`search_channels`](#search_channels), [`list_team_channels`](#list_team_channels), [`list_archived_channels`](#list_archived_channels) | 50 | 200 |

### How a write is checked

Your MCP client's own approval of each call decides whether a change others see
goes ahead: its prompt, its permission rules or its auto mode
([ADR-033](adr/033-the-client-decides-whether-a-write-runs.md)). mm-mcp does
not ask you by default. What it controls is what it checks itself, on every
call, whoever approved it: how far a post or an edit goes, the text an edit
replaces and the reactions and replies that answer it, and what a deletion
takes away, each stated in the call and read again just before the write;
where it goes, by address; and the message itself. A refusal says what mm-mcp
found and tells the model to weigh whether that is what you meant.

A post can also go ahead on your own press. In an app that shows views, a post
your agent proposes with [`propose_post`](#propose_post) shows with Post, which
sends it only while your draft holds what the view shows, after Mattermost's
own question where it would ask one
([Views](views.md#questions-people-ask)).
`MM_MCP_POST_FROM_VIEWS=false` takes Post out of the view.

With `MM_MCP_ELICITATION_OPT_IN=true`, mm-mcp asks you as well
([Configuration](configuration.md)): it shows what will change, where, and under
whose name, and acts only when you tick the box and accept. A question about a
post quotes what it shows: its message, or for one with none, such as an
alert, its blocks or cards, or the names of its files, and names an
integration's post by the name Mattermost shows with the account behind it. A
burn-on-read post you have not opened is not quoted. The question is an
MCP elicitation, a form the client shows, so these tools then need a client that
shows one; a client that declares it cannot gets an error that says so, and
nothing is written. Some clients declare that they can and decline the form
without showing it, as Claude Code does in the Claude desktop app's Code tab.
`diagnose` with `ask_test_question` tells you whether yours shows it.
`MM_MCP_FORCE_HUMAN_IN_THE_LOOP_IN_CLAUDE_CODE` has Claude Code ask on every
call instead.

Every message is checked before the write
([ADR-031](adr/031-a-message-is-checked-before-it-is-posted.md)): one longer
than the server takes is refused with its length and the limit, and an @mention
of someone nobody is is refused with the closest usernames. Where mm-mcp asks,
the question says how many people `@here`, `@channel` and `@all` reach, who
of those mentioned is deactivated, and, on a licensed server with user groups,
how many a group's mention notifies, and puts what a post does, and every file
it carries, before the message. Mentions are found as Mattermost finds them, so
code mentions nobody and `@here.` ending a sentence is `@here`.

#### How far a message goes

`create_post` states in its call how far the message goes, as two numbers
mm-mcp counts itself
([ADR-031](adr/031-a-message-is-checked-before-it-is-posted.md)):

- `reaches`, on every post: how many members the channel has. Everyone in it
  can read the message, whatever it mentions.
- `notifies`, exactly where Mattermost's web app would ask before sending,
  with the number it would show: everyone but you for `@channel`, `@all`
  or `@here` in a channel of more than 5 members, or a mentioned user
  group's members in the channel, above 5. As in Mattermost, it is not asked
  while the server's `EnableConfirmNotificationsToChannel` is off, or of
  someone who may not mention the whole channel.

`update_post` states `reaches` too, never `notifies`. The numbers are in
the call because that is what your MCP client's own approval shows you, or its
AI reviewer, and the model cannot state them without looking. A missing or wrong
number is refused with the real one, and the model is told to weigh whether you
meant the message for that many people before it calls again. Each is counted
again just before the post goes out, so members who joined since the count was
read stop it.

For example, when the model posts `@channel the release is out` in ~Town
Square, a channel of 512 members, without stating the numbers, the call is
refused with:

!!! warning "Refused"

    create_post did not run: a message in ~Town Square reaches 512 people, the
    channel's members as Mattermost counts them, and the call left the number
    out. Consider whether the person meant this message for all of them. If they
    did, call again with reaches: 512. If not, send it somewhere narrower, such as
    a smaller channel or a direct or group message, or ask them where it should go

With `reaches: 512` it is refused again, because Mattermost would ask before
alerting the whole channel:

!!! warning "Refused"

    create_post did not run: @channel alerts 511 people, every member of ~Town
    Square but the user, which Mattermost asks about before sending, and the call
    left the number out. Consider whether the person meant to alert all of them.
    If they did, call again with notifies: 511. If not, mention only the people
    meant, post somewhere narrower, or ask them

The model then calls again with `reaches: 512` and `notifies: 511`, which
your client's approval shows you, as does mm-mcp's question where it asks.
Nothing is posted until both are right.

#### What an edit changes

`update_post` states the post's current text in full, `old_message`, alongside
its replacement
([ADR-031](adr/031-a-message-is-checked-before-it-is-posted.md)).
The model cannot edit a post without writing out all of what it replaces, and
your client's approval shows both texts. An `old_message` that differs from the
post's current text, because you edited the post after the model read it or
because only part of it is quoted, is refused with the current text.

A post with reactions, or with replies in the thread it starts, also takes
`responses`, their number, as `read_post` gives it. An edit leaves those
reactions and replies under the replacing text, and Mattermost tells nobody
about an edit, so a thumbs up given to one sentence stands under another. For
example, an edit of your "Give me a thumbs up if you like cat memes", which has
five 👍 and two replies, that leaves out `responses` is refused with:

!!! warning "Refused"

    update_post did not run: 7 reactions and replies respond to your post in
    ~Town Square: &#58;thumbsup&#58; ×5 (@brent, @ann, @joe, 2 more), 2 replies in its
    thread, and the call left the number out. An edit leaves them under the
    replacing text, and Mattermost tells nobody about it. Consider whether they
    fit the replacement; if the change is news to those who responded, reply in
    the thread instead. If they fit it, call again with responses: 7

A post without reactions or replies takes no `responses`.

#### What a write takes away

A deletion states how much goes in its call, as a number mm-mcp reads itself:
`discards`, the characters of the draft `delete_draft` deletes, as
`list_drafts` gives each draft's length; and `deletes`, the posts
`delete_post` removes, the post and every reply in its thread. A missing or
wrong number is refused with the real one and the start of what would go, and
the model is told to weigh whether you meant to lose it. Each is counted again
just before the write, so a draft you typed on or a reply that arrived since
the model read it stops the write.

For example, deleting a post that started a thread of 12 replies, without
`deletes`:

!!! warning "Refused"

    delete_post did not run: it deletes 13 posts, your post in ~Town Square,
    which begins “Who is in for the release party on Friday? I'll book a table”,
    and the 12 replies in its thread, other people's included, and the call left
    the number out. Consider whether the person meant to delete all of them. If
    they did, call again with deletes: 13. If not, leave it, or ask the person

A thirteenth reply arriving before the model calls again with `deletes: 13`
gets the same refusal, with 14.

`propose_post` takes nothing away. A draft of yours where the proposal would go,
one you wrote or typed in, is left as it is. In an app that shows views, the
view asks you which to keep; in one that does not, the call is refused with what
the draft holds. For example, proposing a reply in ~Town Square, where you
started one of your own:

!!! warning "Refused"

    propose_post did not run: the person has a draft there, which was left as it
    is: “Sounds good, but can we move it to”. If they asked you to change it,
    delete it with delete_draft, stating discards: 34, and propose again. If
    not, they can finish or clear it in Mattermost, or post with create_post

When you asked for your draft to be changed, the model deletes it with
`delete_draft`, whose call states its length, and proposes again.

#### Where a write goes

A write others see names its channel, post or team by its address
([above](#works-the-same-everywhere),
[ADR-030](adr/030-names-are-matched-leniently-and-a-refusal-names-the-next-step.md)),
so your client's approval shows where it goes by name. A post to ~Town Square
named by its channel's name, on a server at chat.example.com, is refused with:

!!! warning "Refused"

    channel_id: ~town-square is a name, but this takes an address,
    mattermost://chat.example.com/&lt;team>/channels/&lt;channel>, or
    &lt;team>/messages/@&lt;username> for a direct message, which names the place
    exactly and shows it to whoever approves the call; get_user_channels,
    get_channel_info and every post give each channel's address

and a reply whose `root_id` names the post by its link:

!!! warning "Refused"

    root_id: https://chat.example.com/eng/pl/4xk3m9p7qfgt8jw1yb5hc2ndre is a link,
    but this takes an address, mattermost://chat.example.com/&lt;team>/channels/&lt;channel>/&lt;post id>:
    its channel's address, then its id, which names the place exactly and shows it
    to whoever approves the call; read_post, read_channel and search_posts give
    each post's address

The model reads the address, `mattermost://chat.example.com/eng/channels/town-square`,
and calls again with it.

Where mm-mcp asks, what a write acts on is what you were asked about: a reply
posted, a file rewritten or a draft typed on while you read the question stops
the write.
Posts and edits are marked as written with AI, as Mattermost shows it, unless
`MM_MCP_MARK_AI_GENERATED` is false. A proposal is not: what you send from
Mattermost is yours.

### On your machine

`save_file`, and attaching a file by its `path`, are offered only when mm-mcp
runs on your own machine, started by your client over stdio, whether writes are
allowed or not: they reach your disk, not Mattermost
([ADR-029](adr/029-files-reach-the-model-as-content-and-the-disk-only-locally.md)).

### Not offered

- **Scheduled posts.** Mattermost's scheduled posts need a licence, and Team
  Edition, which mm-mcp is tested against, refuses them
  ([ADR-004](adr/004-live-tests-against-a-real-mattermost.md)).
- **Marking what the model read as read, or anything unread.** What you have
  read is yours to record, and a model reading a channel is not you reading
  it; `mark_channel_read` marks a channel read when you ask
  ([ADR-021](adr/021-read-only-by-default.md)).

### How the tools are built

Every tool declares a title and the four MCP annotations: whether it only reads,
whether it may destroy something, whether repeating it changes nothing more, and
that it works in a closed domain, your one Mattermost server, which the MCP
specification counts as closed however freely people write in it
([ADR-021](adr/021-read-only-by-default.md)). They are
hints: a client may use them to decide what to ask you about, or ignore them,
and none of them is a security control. Some clients run a tool marked
read-only without asking you or their AI reviewer, so only a tool that changes
nothing in Mattermost is marked so, and a test holds each one to the endpoints
it calls.

Each tool also accounts for every parameter of the Mattermost endpoints it
calls: which argument sets it, or why it is fixed or left out
([ADR-028](adr/028-every-parameter-of-an-operation-a-tool-calls-is-accounted-for.md)),
and a live test calls every tool against a real Mattermost
([ADR-004](adr/004-live-tests-against-a-real-mattermost.md)).
