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

// isSystem says whether Mattermost wrote the post, such as someone joining
// the channel. A post of another type, such as one an integration posted with
// message attachments, is written by its author like any other.
function isSystem(post) {
  return typeof post.type === "string" && post.type.startsWith("system_");
}

// continues says whether a post runs on from the one before it, under one
// header, as Mattermost runs them: by one author within minutes, and never a
// post an integration made, nor one written with AI.
function continues(previous, post) {
  if (!previous || previous.author_id !== post.author_id || isSystem(previous) || isSystem(post)) return false;
  if (previous.from_integration || post.from_integration || previous.ai_generated || post.ai_generated) return false;
  const before = wallTime(previous.created_at);
  const now = wallTime(post.created_at);
  return Boolean(before && now && now - before < SAME_RUN_MS);
}

function renderPost(post, options) {
  const classes = ["post"];
  if (options.asked) classes.push("asked");
  if (options.continued) classes.push("continued");
  if (isSystem(post)) classes.push("system");
  // An integration's post shows the name it chose, and its own icon where
  // Mattermost shows one, which the view has no picture of.
  const person = {
    id: post.integration_icon ? "" : post.author_id,
    username: post.author,
    name: post.shown_as || post.author_name || post.author,
  };
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
      el("span", { class: "author", title: post.shown_as && post.author ? "@" + post.author : null }, person.name),
      !post.shown_as && post.author_name && post.author ? el("span", { class: "muted username" }, "@" + post.author) : null,
      post.from_integration || post.author_is_bot ? el("span", { class: "bot-tag" }, t("post.bot")) : null,
      time,
      marks(post)));
  } else if (marks(post).length > 0) {
    body.append(el("div", { class: "post-header" }, marks(post)));
  }
  const markdown = { base: options.place.base, team: options.place.teamURL };
  // A burn-on-read post the person has not revealed shows nothing it holds.
  const concealed = post.concealed === true;
  const cards = !concealed && Array.isArray(post.message_attachments) ? post.message_attachments : [];
  const blocks = !concealed && Array.isArray(post.blocks) ? post.blocks : [];
  if (concealed) {
    body.append(concealedPlaceholder(post));
  } else if (isSystem(post)) {
    body.append(el("p", { class: "system-message muted" }, post.message));
  } else if (post.message || (cards.length === 0 && blocks.length === 0)) {
    body.append(renderMarkdown(post.message, markdown));
  }
  if (post.edited_at) {
    body.append(el("span", { class: "edited muted", title: t("post.edited_at", { time: formatMoment(post.edited_at) }) }, t("post.edited")));
  }
  if (post.message_length) {
    body.append(el("p", { class: "muted cut" },
      t("post.cut", { count: post.message_length }), " ",
      linkButton(post.url, [t("post.read_whole")])));
  }
  if (cards.length > 0) body.append(el("div", { class: "attachments" }, cards.map((card) => messageAttachment(card, markdown))));
  if (post.message_attachments_length) {
    body.append(el("p", { class: "muted cut" },
      t("post.cut_attachments", { count: post.message_attachments_length }), " ",
      linkButton(post.url, [t("post.read_whole")])));
  }
  if (blocks.length > 0) body.append(renderBlocks(blocks, markdown));
  if (post.blocks_length) {
    body.append(el("p", { class: "muted cut" },
      t("post.cut_blocks", { count: post.blocks_length }), " ",
      linkButton(post.url, [t("post.read_whole")])));
  }
  if (!concealed && isObject(post.quoted_post)) body.append(renderQuote(post.quoted_post, markdown));
  if (!concealed && isObject(post.link_preview)) body.append(renderLinkPreview(post.link_preview, markdown));
  if (!concealed && Array.isArray(post.files) && post.files.length > 0) body.append(renderFiles(post));
  const acknowledgements = post.requested_ack === true ? renderAcknowledgements(post) : null;
  const reactions = !concealed && post.reactions && typeof post.reactions === "object" ? renderReactions(post.reactions) : null;
  if (acknowledgements || reactions) body.append(el("div", { class: "responses" }, acknowledgements, reactions));

  return el("li", { class: classes.join(" "), dataset: { post: post.id } },
    // A post that carries on a run shows its time where the picture would be,
    // when the pointer or the keyboard is on it, as in Mattermost.
    el("div", { class: "post-side" }, options.continued ? time : avatar(person)),
    body);
}

// priorityLabel is the label Mattermost shows after the time of a post its
// author marked Important or Urgent, with its icon.
function priorityLabel(priority) {
  if (priority === "urgent") return [icon("alert"), t("post.priority_urgent")];
  if (priority === "important") return [icon("alert-circle"), t("post.priority_important")];
  return null;
}

function marks(post) {
  const found = [];
  const priority = priorityLabel(post.priority);
  if (priority) found.push(el("span", { class: "priority-tag", dataset: { priority: post.priority } }, priority));
  // A burn-on-read post is marked, with when it burns for the person in its
  // title, as Mattermost's badge and timer show it.
  if (post.type === "burn_on_read") {
    found.push(el("span", { class: "mark burn", title: post.expire_at ? t("post.burns_at", { time: formatMoment(post.expire_at) }) : null },
      icon("flame"), t("post.burn_on_read")));
  }
  if (post.pinned) found.push(el("span", { class: "mark pinned" }, icon("pin"), t("post.pinned")));
  if (post.ai_generated) found.push(el("span", { class: "mark muted" }, t("post.ai")));
  return found;
}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

// renderQuote is the post a post links to, quoted below its message as
// Mattermost quotes it: its author, under the name an integration chose,
// when, its label, its message, its own preview, and the channel it was
// posted in. Its time opens it in Mattermost.
function renderQuote(quote, markdown) {
  const text = (value) => typeof value === "string" ? value : "";
  const at = text(quote.created_at);
  const files = Array.isArray(quote.files) ? quote.files.length : 0;
  const priority = priorityLabel(quote.priority);
  return el("blockquote", { class: "quote" },
    el("div", { class: "post-header" },
      el("span", { class: "author", title: text(quote.shown_as) && text(quote.author) ? "@" + quote.author : null },
        text(quote.shown_as) || text(quote.author_name) || text(quote.author)),
      linkButton(text(quote.url), [at ? formatTime(at) : t("quote.open")], { class: "link time", "aria-label": t("quote.open") }),
      priority ? el("span", { class: "priority-tag", dataset: { priority: quote.priority } }, priority) : null,
      quote.ai_generated === true ? el("span", { class: "mark muted" }, t("post.ai")) : null),
    renderMarkdown(text(quote.message), markdown),
    quote.message_length ? el("p", { class: "muted cut" }, t("post.cut", { count: quote.message_length }), " ", linkButton(text(quote.url), [t("post.read_whole")])) : null,
    files > 0 ? el("div", { class: "muted quote-files" }, icon("file"), t("quote.files", { count: files })) : null,
    isObject(quote.link_preview) ? renderLinkPreview(quote.link_preview, markdown) : null,
    text(quote.channel) ? el("div", { class: "muted quote-channel" }, t("quote.channel", { channel: quote.channel })) : null);
}

// renderLinkPreview is the page or picture a post's first link leads to, as
// Mattermost shows it below the message: a page's site, its title linking to
// it and its description, and a picture named rather than drawn, as an image
// in a message is, since the view has no network.
function renderLinkPreview(preview, markdown) {
  const text = (value) => typeof value === "string" ? value : "";
  if (preview.image === true) {
    return el("div", { class: "link-preview link-image" }, namedImage(text(preview.url), markdown));
  }
  if (!text(preview.title) && !text(preview.description) && !text(preview.site_name)) return null;
  return el("div", { class: "link-preview" },
    text(preview.site_name) ? el("div", { class: "muted link-site" }, preview.site_name) : null,
    text(preview.title) ? el("h3", { class: "link-title" }, linkButton(text(preview.url), [preview.title])) : null,
    text(preview.description) ? el("p", { class: "link-description" }, preview.description) : null);
}

// namedImage is a picture an address leads to, named as given or after its
// file, which opens through the host on a click, as an image in a message
// does.
function namedImage(url, markdown, given) {
  let name = typeof given === "string" ? given : "";
  try {
    name = name || decodeURIComponent(new URL(url).pathname.split("/").pop() || "");
  } catch {
    name = name || "";
  }
  return markdownImage(name, url, { base: markdown.base });
}

// BLOCKS_DRAWN is the most blocks the view draws of a post, as many as
// Mattermost keeps.
const BLOCKS_DRAWN = 100;

// renderBlocks draws what an integration posted as blocks or cards, as
// Mattermost draws them. They come as an outline: a container, collapsible,
// column set or column holds the blocks after it that are deeper.
function renderBlocks(blocks, markdown) {
  const root = el("div", { class: "blocks" });
  const open = [{ depth: -1, holder: root }];
  for (const block of blocks.slice(0, BLOCKS_DRAWN)) {
    if (!isObject(block)) continue;
    const depth = Number.isInteger(block.depth) && block.depth > 0 ? block.depth : 0;
    while (open.length > 1 && open[open.length - 1].depth >= depth) open.pop();
    const drawn = drawBlock(block, markdown);
    if (!drawn) continue;
    open[open.length - 1].holder.append(drawn.node);
    if (drawn.holds) open.push({ depth, holder: drawn.node });
  }
  return root;
}

// drawBlock is one block, and whether it holds the blocks after it. A
// container's bar is the view's color nearest the one the integration chose,
// a collapsible is folded where Mattermost folds it, and an image is named
// rather than drawn.
function drawBlock(block, markdown) {
  const text = (value) => typeof value === "string" ? value : "";
  switch (block.type) {
  case "text":
    return { node: el("div", { class: block.subtle === true ? "block-text muted" : "block-text" }, renderMarkdown(text(block.text), markdown)) };
  case "image":
    return { node: el("div", { class: "block-image", title: text(block.title) || null }, namedImage(text(block.url), markdown, text(block.alt_text))) };
  case "divider":
    return { node: el("hr", { class: "block-divider" }) };
  case "container":
    return { node: el("div", { class: "block-container", dataset: { bar: blockBar(block.accent_color), border: block.border === true ? "true" : "false" } }), holds: true };
  case "collapsible":
    return { node: el("details", { class: "block-collapsible", open: block.collapsed !== true }, el("summary", {}, renderMarkdown(text(block.text), markdown))), holds: true };
  case "column_set":
    return { node: el("div", { class: "block-columns" }), holds: true };
  case "column":
    return { node: el("div", { class: "block-column" }), holds: true };
  }
  return null;
}

// blockBar is the view's color for a container's bar: the two Mattermost
// names for blocks alone, and otherwise as a card's.
function blockBar(color) {
  if (color === "primary") return "blue";
  if (color === "default") return "gray";
  return barOf(color);
}

// concealedPlaceholder stands in for a burn-on-read post the person has not
// revealed, as Mattermost conceals it. Revealing it is theirs to do, in
// Mattermost: it starts the timer after which the post is gone for them.
function concealedPlaceholder(post) {
  return el("div", { class: "concealed", role: "note" },
    icon("flame"),
    el("span", { class: "concealed-text" }, t("post.concealed")),
    linkButton(post.url, [t("post.view_message")], { class: "link concealed-open" }));
}

// messageAttachment is a card an integration posted, as Mattermost draws it:
// its pretext above it, and in it, beside a colored bar, its author, its title,
// each linking where the integration says, its thumbnail, its text, its
// fields, two short ones to a row, its picture, and its footer. Its pictures
// are named rather than drawn, since the view has no network. Its fallback is what a notification shows,
// and is not drawn, and a card with nothing else is not drawn either.
function messageAttachment(card, markdown) {
  const text = (value) => typeof value === "string" ? value : "";
  const parts = [
    text(card.author_name) ? el("div", { class: "attachment-author" }, linkButton(text(card.author_link), [card.author_name])) : null,
    text(card.title) ? el("h3", { class: "attachment-title" }, linkButton(text(card.title_link), [card.title])) : null,
    text(card.thumb_url) ? el("div", { class: "attachment-thumb" }, namedImage(card.thumb_url, markdown)) : null,
    text(card.text) ? renderMarkdown(card.text, markdown) : null,
    attachmentFields(card.fields, markdown),
    text(card.image_url) ? el("div", { class: "attachment-image" }, namedImage(card.image_url, markdown)) : null,
    text(card.footer) ? el("div", { class: "attachment-footer muted" }, card.footer) : null,
  ].filter(Boolean);
  return el("div", { class: "attachment" },
    text(card.pretext) ? renderMarkdown(card.pretext, markdown) : null,
    parts.length > 0 ? el("div", { class: "attachment-card", dataset: { bar: barOf(card.color) } }, parts) : null);
}

// attachmentFields lays out fields as Mattermost does: a short field shares
// its row with the next short one, and a long one has a row of its own.
function attachmentFields(fields, markdown) {
  if (!Array.isArray(fields)) return null;
  const rows = [];
  let row = [];
  let long = false;
  for (const field of fields) {
    if (!field || typeof field !== "object") continue;
    if (row.length === 2 || field.short !== true || long) {
      if (row.length > 0) rows.push(row);
      row = [];
    }
    row.push(field);
    long = field.short !== true;
  }
  if (row.length > 0) rows.push(row);
  if (rows.length === 0) return null;
  return el("div", { class: "attachment-fields" }, rows.map((cells) => el("table", { class: "attachment-row" },
    el("thead", {}, el("tr", {}, cells.map((field) => el("th", {}, typeof field.title === "string" ? field.title : "")))),
    el("tbody", {}, el("tr", {}, cells.map((field) => el("td", {}, renderMarkdown(typeof field.value === "string" ? field.value : "", markdown))))))));
}

// ATTACHMENT_BARS are the colors Mattermost names for a card's bar, by the
// fixed color of the view's own that draws each.
const ATTACHMENT_BARS = { good: "green", warning: "yellow", danger: "red" };

const HEX_COLOR = /^#([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;

// barOf is the view's color for a card's bar. Colors never come from
// Mattermost content, so a hex color an integration chose is drawn in the
// view's color nearest its hue, which keeps red apart from green, firing from
// resolved: gray when it has little color, and none when it is no color.
function barOf(color) {
  if (typeof color !== "string") return "none";
  if (Object.hasOwn(ATTACHMENT_BARS, color)) return ATTACHMENT_BARS[color];
  const hex = HEX_COLOR.exec(color);
  if (!hex) return "none";
  let digits = hex[1];
  if (digits.length <= 4) digits = digits.split("").map((digit) => digit + digit).join("");
  const [r, g, b] = [0, 2, 4].map((at) => parseInt(digits.slice(at, at + 2), 16) / 255);
  const high = Math.max(r, g, b);
  const low = Math.min(r, g, b);
  if (high - low < 0.15) return "gray";
  let hue;
  if (high === r) hue = ((g - b) / (high - low) + 6) % 6;
  else if (high === g) hue = (b - r) / (high - low) + 2;
  else hue = (r - g) / (high - low) + 4;
  hue *= 60;
  if (hue < 15 || hue >= 330) return "red";
  if (hue < 40) return "orange";
  if (hue < 70) return "yellow";
  if (hue < 170) return "green";
  if (hue < 260) return "blue";
  return "purple";
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

// renderAcknowledgements is the count of people who acknowledged a post that
// asks for it, with who in its title, as reactions name who reacted. Pressing
// it in Mattermost acknowledges the post, which here is not the view's to do.
function renderAcknowledgements(post) {
  const given = Array.isArray(post.acknowledgements) ? post.acknowledgements.filter((ack) => ack && typeof ack.user === "string") : [];
  const who = given.map((ack) => "@" + ack.user).join(", ");
  return el("span", {
    class: "acknowledgements",
    dataset: { acknowledged: post.acknowledged === true ? "true" : "false" },
    title: given.length > 0 ? t("acknowledgements.who", { count: given.length, who }) : t("acknowledgements.none"),
  }, icon("check"), el("span", { class: "acknowledgement-count" }, formatNumber(given.length)));
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
