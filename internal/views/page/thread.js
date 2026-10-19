// A post, alone or with its thread, laid out as Mattermost lays it out. The
// data is what read_post answers (ADR-022): the post asked for, and the
// thread's first post and replies, oldest first, a page at a time. Counts are
// Mattermost's, and a thread that lists fewer replies than it has says so.

// MORE_REPLIES is how many replies "more" asks for at a time.
const MORE_REPLIES = 30;

// A run of posts by one person within this long shows their name once, as in
// Mattermost.
const SAME_RUN_MS = 5 * 60 * 1000;

function renderThread(data) {
  const asked = data.post || null;
  const withThread = Array.isArray(data.thread) && data.thread.length > 0;
  const posts = withThread ? data.thread : asked ? [asked] : [];
  if (posts.length === 0) return el("p", { class: "muted", role: "status" }, t("thread.empty"));
  const place = placeOf(asked || posts[0]);
  const root = posts.find((post) => post.id === data.root_id) || null;
  const replies = root ? posts.filter((post) => post.id !== root.id).length : 0;
  const total = root && typeof root.reply_count === "number" ? root.reply_count : replies;

  const list = el("ol", { class: "posts", "aria-label": t("thread.posts") });
  let previous = null;
  for (const post of posts) {
    if (!previous || dayOf(previous.created_at) !== dayOf(post.created_at)) {
      list.append(el("li", { class: "day", role: "separator" }, el("span", {}, formatDay(post.created_at))));
      previous = null;
    }
    list.append(renderPost(post, {
      // The post asked for stands out when it is a reply in the thread, as a
      // permalink to it does in Mattermost.
      asked: Boolean(asked && post.id === asked.id && asked.id !== data.root_id),
      continued: continues(previous, post),
      place,
    }));
    previous = post;
  }

  const shown = replies;
  return el("article", { class: "thread" },
    threadHeader(place, root, total),
    list,
    root && withThread && total > shown ? moreReplies(data, root, total - shown) : null,
    root && !withThread && total > 0 ? el("div", { class: "more-row" }, linkButton(root.url, [t("thread.open")], { class: "link muted" })) : null);
}

function threadHeader(place, root, total) {
  return el("header", { class: "thread-header" },
    el("h2", { class: "thread-title" },
      place.channel ? linkButton(place.channelURL, [place.channel], { class: "link channel-name" }) : t("thread.title")),
    place.team ? el("span", { class: "muted team" }, place.team) : null,
    root ? el("span", { class: "muted count" }, t("thread.replies", { count: total })) : null);
}

// placeOf is where the posts are: their channel and team, and the addresses
// links in them are taken against.
function placeOf(post) {
  const place = { channel: post.channel || "", channelURL: post.channel_url || "", team: post.team || "", base: "", teamURL: "" };
  try {
    place.base = new URL(post.url).origin;
  } catch {
    place.base = "";
  }
  const team = /^(.*)\/(?:channels|messages)\/[^/]+$/.exec(place.channelURL);
  if (team && isWebURL(team[1])) place.teamURL = team[1];
  return place;
}

function continues(previous, post) {
  if (!previous || previous.author_id !== post.author_id || previous.type || post.type) return false;
  const before = wallTime(previous.created_at);
  const now = wallTime(post.created_at);
  return Boolean(before && now && now - before < SAME_RUN_MS);
}

function renderPost(post, options) {
  const classes = ["post"];
  if (options.asked) classes.push("asked");
  if (options.continued) classes.push("continued");
  if (post.type) classes.push("system");
  const person = { id: post.author_id, username: post.author, name: post.author_name || post.author };
  const time = el("button", {
    type: "button",
    class: "link time",
    title: formatMoment(post.created_at),
    "aria-label": t("post.open", { time: formatMoment(post.created_at) }),
    onclick: () => openLink(post.url),
  }, formatTime(post.created_at));

  const body = el("div", { class: "post-body" });
  if (!options.continued) {
    body.append(el("div", { class: "post-header" },
      el("span", { class: "author" }, person.name),
      post.author_name && post.author ? el("span", { class: "muted username" }, "@" + post.author) : null,
      time,
      marks(post)));
  } else if (marks(post).length > 0) {
    body.append(el("div", { class: "post-header" }, marks(post)));
  }
  if (post.type) {
    body.append(el("p", { class: "system-message muted" }, post.message));
  } else {
    body.append(renderMarkdown(post.message, { base: options.place.base, team: options.place.teamURL }));
  }
  if (post.edited_at) {
    body.append(el("span", { class: "edited muted", title: t("post.edited_at", { time: formatMoment(post.edited_at) }) }, t("post.edited")));
  }
  if (post.message_length) {
    body.append(el("p", { class: "muted cut" },
      t("post.cut", { count: post.message_length }), " ",
      linkButton(post.url, [t("post.read_whole")])));
  }
  if (Array.isArray(post.files) && post.files.length > 0) body.append(renderFiles(post));
  if (post.reactions && typeof post.reactions === "object") body.append(renderReactions(post.reactions));

  return el("li", { class: classes.join(" "), dataset: { post: post.id } },
    // A post that carries on a run shows its time where the picture would be,
    // when the pointer or the keyboard is on it, as in Mattermost.
    el("div", { class: "post-side" }, options.continued ? time : avatar(person)),
    body);
}

function marks(post) {
  const found = [];
  if (post.pinned) found.push(el("span", { class: "mark pinned" }, icon("pin"), t("post.pinned")));
  if (post.ai_generated) found.push(el("span", { class: "mark muted" }, t("post.ai")));
  return found;
}

function renderFiles(post) {
  return el("div", { class: "files" }, post.files.map((file) => file.preview_uri ? imageCard(file) : fileCard(file, post)));
}

// imageCard is an image as its post shows it: its preview, and the whole
// image on a click.
function imageCard(file) {
  const name = file.name || t("file.unnamed");
  const frame = el("button", {
    type: "button",
    class: "image-card",
    "aria-label": t("file.view_image", { name }),
    title: name,
    onclick: () => showImageWhole(file),
  }, el("span", { class: "image-placeholder" }, icon("image"), name));
  if (file.width > 0 && file.height > 0) {
    frame.dataset.sized = "true";
    frame.setAttribute("aria-description", t("file.dimensions", { width: file.width, height: file.height }));
  }
  readPicture(file.preview_uri).then((url) => {
    if (!url) return;
    // The image's own size, so the picture keeps its shape however large
    // the preview is.
    const img = el("img", { alt: name, src: url, width: file.width > 0 ? file.width : undefined, height: file.height > 0 ? file.height : undefined });
    if (img.hasAttribute("src")) frame.replaceChildren(img);
  });
  return frame;
}

function showImageWhole(file) {
  const name = file.name || t("file.unnamed");
  const close = el("button", { type: "button", class: "close", "aria-label": t("image.close") }, "×");
  const holder = el("div", { class: "lightbox-image" }, el("span", { class: "muted", role: "status" }, t("image.loading")));
  const box = el("div", { class: "lightbox", role: "dialog", "aria-modal": "true", "aria-label": name, tabindex: "-1" },
    el("div", { class: "lightbox-bar" }, el("span", { class: "ellipsis" }, name), close),
    holder);
  const done = () => {
    view.lightbox = null;
    render();
  };
  close.addEventListener("click", done);
  box.addEventListener("keydown", (event) => {
    if (event.key === "Escape") done();
  });
  readPicture(file.uri).then((url) => {
    const img = url ? el("img", { alt: name, src: url }) : null;
    holder.replaceChildren(img && img.hasAttribute("src") ? img : el("span", { class: "muted" }, t("image.unavailable")));
  });
  view.lightbox = box;
  render();
  close.focus();
}

function fileCard(file, post) {
  return el("div", { class: "file-card" },
    icon("file"),
    el("span", { class: "file-text" },
      el("span", { class: "file-name ellipsis" }, file.name || t("file.unnamed")),
      el("span", { class: "muted file-meta" }, [file.mime_type, file.size > 0 ? formatSize(file.size) : ""].filter(Boolean).join(" · "))),
    linkButton(post.url, [icon("open", t("file.open"))], { class: "link icon-button", "aria-label": t("file.open") }));
}

function renderReactions(reactions) {
  const names = Object.keys(reactions);
  if (names.length === 0) return null;
  return el("ul", { class: "reactions", "aria-label": t("reactions.label") }, names.map((name) => {
    const people = Array.isArray(reactions[name]) ? reactions[name] : [];
    const who = people.map((person) => "@" + person).join(", ");
    return el("li", { class: "reaction", title: t("reactions.who", { count: people.length, who, emoji: ":" + name + ":" }) },
      emoji(name), el("span", { class: "reaction-count" }, formatNumber(people.length)));
  }));
}

// moreReplies offers the replies the page left out, a page at a time,
// through the tool only views call; where the host refuses, the thread opens
// in Mattermost.
function moreReplies(data, root, left) {
  const button = el("button", { type: "button", class: "more" }, icon("more"), t("thread.more", { count: left }));
  button.addEventListener("click", async () => {
    if (!data.next_cursor || !view.callsTools) {
      openLink(root.url);
      return;
    }
    button.disabled = true;
    try {
      const result = await bridge.callTool("view_action", { action: "more", post_id: root.id, cursor: data.next_cursor, limit: MORE_REPLIES });
      const page = result && !result.isError ? result.structuredContent : null;
      if (!page || !Array.isArray(page.thread)) throw new Error("no page");
      const known = new Set(data.thread.map((post) => post.id));
      data.thread = data.thread.concat(page.thread.filter((post) => !known.has(post.id)));
      data.next_cursor = page.next_cursor || "";
      render();
    } catch {
      button.disabled = false;
      notice(t("thread.more_refused"), root.url);
    }
  });
  return el("div", { class: "more-row" },
    button,
    linkButton(root.url, [t("thread.open")], { class: "link muted" }));
}

// renderPosted is a post as posted, as create_post, dm and group_message
// answer with it: the post alone, under where it went, and for a reply, the
// way to its thread.
function renderPosted(post) {
  const place = placeOf(post);
  return el("article", { class: "thread posted" },
    el("header", { class: "thread-header" },
      el("span", { class: "muted posted-label" }, post.root_id ? t("post.replied") : t("post.posted")),
      el("h2", { class: "thread-title" },
        place.channel ? linkButton(place.channelURL, [place.channel], { class: "link channel-name" }) : t("thread.title")),
      place.team ? el("span", { class: "muted team" }, place.team) : null),
    el("ol", { class: "posts", "aria-label": t("thread.posts") }, renderPost(post, { asked: false, continued: false, place })),
    post.root_id ? el("div", { class: "more-row" }, linkButton(post.url, [t("thread.open")], { class: "link muted" })) : null);
}
