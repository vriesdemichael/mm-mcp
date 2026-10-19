// What is unread for the person, as Mattermost counts it: their channels with
// something unread, grouped by team, the posts that mention them, and the
// threads they follow with unread replies. The data is what get_user_channels
// with unread_only, list_mentions and list_threads with unread_only answer
// (ADR-022), a page of each; where Mattermost has more, the view says so.
//
// A channel's unread messages, and a thread, open in place through the tool
// that only views call and changes nothing, or in Mattermost where the host
// refuses it. Mark read goes through the one that changes something, which
// the server offers only where writes are allowed (CHANGES).

// UNREAD_PAGE is how many posts a channel opened in place shows.
const UNREAD_PAGE = 30;

// opened is what is open in place: one channel or thread at a time.
const opened = { key: "", kind: "", data: null };

function renderUnreads(data) {
  const channels = data.channels && Array.isArray(data.channels.channels) ? data.channels.channels : [];
  const mentions = data.mentions && Array.isArray(data.mentions.posts) ? data.mentions.posts : [];
  const threads = data.threads && Array.isArray(data.threads.threads) ? data.threads.threads : [];
  if (channels.length === 0 && mentions.length === 0 && threads.length === 0) {
    return el("p", { class: "muted", role: "status" }, t("unreads.nothing"));
  }
  return el("article", { class: "unreads" },
    channels.length > 0 ? section("channels", t("unreads.channels"), channelGroups(channels), data.channels.next_cursor ? t("unreads.more_channels") : "") : null,
    mentions.length > 0 ? section("mentions", t("unreads.mentions"), el("ul", { class: "unread-list" }, mentions.map(mentionRow)), data.mentions.next_cursor ? t("unreads.more_mentions") : "") : null,
    threads.length > 0 ? section("threads", t("unreads.threads"), el("ul", { class: "unread-list" }, threads.map(threadRow)), data.threads.next_cursor ? t("unreads.more_threads") : "") : null);
}

function section(name, title, list, more) {
  return el("section", { class: "unread-section", "aria-label": title },
    el("h2", { class: "unread-heading" }, title),
    list,
    more ? el("p", { class: "muted more-note" }, more) : null);
}

// channelGroups lists the channels by team, in Mattermost's order: most
// recently active first. Direct and group messages, which belong to no team,
// are a group of their own.
function channelGroups(channels) {
  const groups = new Map();
  for (const channel of channels) {
    const team = channel.type === "direct" || channel.type === "group" ? t("unreads.direct") : channel.team || t("unreads.direct");
    if (!groups.has(team)) groups.set(team, []);
    groups.get(team).push(channel);
  }
  return el("div", { class: "unread-groups" }, [...groups].map(([team, listed]) =>
    el("div", { class: "unread-group" },
      el("h3", { class: "muted unread-team" }, team),
      el("ul", { class: "unread-list" }, listed.map(channelRow)))));
}

function channelRow(channel) {
  const key = "channel:" + channel.id;
  const unread = typeof channel.unread_messages === "number" ? channel.unread_messages : 0;
  const mentioned = typeof channel.mentions === "number" ? channel.mentions : 0;
  const name = channel.type === "direct" ? "@" + channel.display_name : channel.display_name;
  const row = el("li", { class: "unread-row" + (channel.marked ? " marked" : "") },
    el("div", { class: "unread-line" },
      el("button", {
        type: "button",
        class: "link unread-name ellipsis",
        "aria-expanded": String(opened.key === key),
        onclick: () => openInPlace(key, "channel", channel.url, { action: "unread", channel_id: channel.id, limit: UNREAD_PAGE }),
      }, name),
      channel.marked
        ? el("span", { class: "muted unread-count" }, t("unreads.marked"))
        : [
          mentioned > 0 ? el("span", { class: "badge", title: t("unreads.mentioned", { count: mentioned }) }, "@" + formatNumber(mentioned)) : null,
          el("span", { class: "muted unread-count" }, t("unreads.unread", { count: unread })),
          markRead(channel),
        ]));
  if (opened.key === key && opened.data) row.append(openedChannel(opened.data, channel));
  return row;
}

// markRead offers Mark read, where the server lets a view mark a channel read
// and the host lets the view call a tool.
function markRead(channel) {
  if (!CHANGES || !view.callsTools) return null;
  const button = el("button", { type: "button", class: "link muted mark-read" }, t("unreads.mark_read"));
  button.addEventListener("click", async () => {
    button.disabled = true;
    try {
      const result = await bridge.callTool("view_change", { action: "mark_read", channel_id: channel.id });
      if (!result || result.isError) throw new Error("refused");
      channel.marked = true;
      render();
    } catch {
      button.disabled = false;
      notice(t("unreads.mark_refused"), channel.url);
    }
  });
  return button;
}

function mentionRow(post) {
  const key = "mention:" + post.id;
  const row = el("li", { class: "unread-row" },
    el("div", { class: "unread-line" },
      el("button", {
        type: "button",
        class: "link unread-name ellipsis",
        "aria-expanded": String(opened.key === key),
        onclick: () => openInPlace(key, "thread", post.url, { action: "thread", post_id: post.id, limit: MORE_REPLIES }),
      }, post.channel || t("unreads.direct")),
      el("span", { class: "muted unread-count", title: formatMoment(post.created_at) }, formatTime(post.created_at))),
    el("ol", { class: "posts compact" }, renderPost(post, { asked: false, continued: false, place: placeOf(post) })));
  if (opened.key === key && opened.data) row.append(openedThread(opened.data));
  return row;
}

function threadRow(thread) {
  const key = "thread:" + thread.root_id;
  const started = thread.started || {};
  const unread = typeof thread.unread_replies === "number" ? thread.unread_replies : 0;
  const mentioned = typeof thread.unread_mentions === "number" ? thread.unread_mentions : 0;
  const row = el("li", { class: "unread-row" },
    el("div", { class: "unread-line" },
      el("button", {
        type: "button",
        class: "link unread-name ellipsis",
        "aria-expanded": String(opened.key === key),
        onclick: () => openInPlace(key, "thread", started.url, { action: "thread", post_id: thread.root_id, limit: MORE_REPLIES }),
      }, started.channel || t("unreads.direct")),
      mentioned > 0 ? el("span", { class: "badge", title: t("unreads.mentioned", { count: mentioned }) }, "@" + formatNumber(mentioned)) : null,
      el("span", { class: "muted unread-count" }, t("unreads.unread_replies", { count: unread }))),
    el("p", { class: "unread-excerpt ellipsis" }, started.message || ""),
    Array.isArray(thread.participants) && thread.participants.length > 0
      ? el("p", { class: "muted unread-people ellipsis" }, thread.participants.map((person) => "@" + person).join(", "))
      : null);
  if (opened.key === key && opened.data) row.append(openedThread(opened.data));
  return row;
}

// openInPlace opens a channel or a thread under its row through the tool
// that only views call, or closes it when it is open. Where the host refuses
// the call, the view says so and offers it in Mattermost.
async function openInPlace(key, kind, url, args) {
  if (opened.key === key) {
    opened.key = "";
    opened.data = null;
    render();
    return;
  }
  if (!view.callsTools) {
    openLink(url);
    return;
  }
  try {
    const result = await bridge.callTool("view_action", args);
    const data = result && !result.isError ? result.structuredContent : null;
    if (!data || typeof data !== "object") throw new Error("nothing came back");
    opened.key = key;
    opened.kind = kind;
    opened.data = data;
    render();
  } catch {
    notice(t("unreads.open_refused"), url);
  }
}

// openedChannel is a channel's unread messages, as read_unread answers them:
// a few the person has read, then the ones they have not.
function openedChannel(data, channel) {
  const posts = Array.isArray(data.posts) ? data.posts : [];
  const place = placeOf({ channel: data.channel || channel.display_name, channel_url: data.channel_url || channel.url, team: data.team || channel.team, url: posts.length > 0 ? posts[0].url : channel.url });
  const list = el("ol", { class: "posts", "aria-label": t("thread.posts") });
  let previous = null;
  for (const post of posts) {
    if (post.id === data.first_unread_id) {
      list.append(el("li", { class: "day new-messages", role: "separator" }, el("span", {}, t("unreads.new_messages"))));
      previous = null;
    }
    list.append(renderPost(post, { asked: false, continued: continues(previous, post), place }));
    previous = post;
  }
  return el("div", { class: "opened" },
    list,
    el("div", { class: "more-row" }, linkButton(channel.url, [t("unreads.open_channel")], { class: "link muted" })));
}

// openedThread is a thread, as read_post answers it, drawn as the thread view
// draws one.
function openedThread(data) {
  return el("div", { class: "opened" }, renderThread(data));
}
