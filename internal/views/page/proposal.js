// A post the model proposed, drawn as it will be posted, waiting as the
// person's draft in Mattermost (ADR-035). The data is what propose_post
// answers: the draft and where it waits, or, where a draft of the person's
// own is in the way, both, for them to choose between.
//
// Every press goes through the tool only views call that changes something,
// which the server offers where writes are allowed (CHANGES), and names the
// text the view shows: mm-mcp acts only while the draft still holds it, and a
// draft changed since is drawn again as it is. Post is offered only where the
// server posts from a view (POSTS); elsewhere the draft waits in Mattermost
// for the person to send. The preview hides nothing that is sent: a link
// shows where it leads, a character the eye cannot see is flagged, and the
// text as written is a click away. What the person did reaches the model
// through Mattermost, and, where the host passes it on, through
// ui/update-model-context in mm-mcp's fixed words; never as a message in
// their name.

// proposed is the proposal the view shows, and what the person is doing with
// it. A result drawn again, as a host does when a conversation is reopened,
// starts it afresh from that result.
const proposed = {
  source: null,
  draft: null,
  theirs: null,
  stage: "",
  text: "",
  written: false,
  confirm: null,
  post: null,
  busy: false,
  // A Post that got no answer may have posted: it is not offered again.
  unsure: false,
};

// WHOLE_CHANNEL are the mentions that alert everyone in a channel.
const WHOLE_CHANNEL = ["@all", "@channel", "@here"];

function renderProposal(data) {
  if (proposed.source !== data) {
    Object.assign(proposed, {
      source: data,
      draft: data,
      theirs: data.saved === false && data.theirs ? data.theirs : null,
      stage: data.saved === false && data.theirs ? "choosing" : "proposed",
      text: "",
      written: false,
      confirm: null,
      post: null,
      busy: false,
      unsure: false,
    });
  }
  const draft = proposed.draft;
  switch (proposed.stage) {
    case "posted":
      return renderPosted(proposed.post);
    case "choosing":
      return choosing(draft, proposed.theirs);
    case "editing":
      return editing(draft);
    case "discarded":
      return ended(draft, t("proposal.discarded"), draft.channel_url);
    case "kept":
      return ended(draft, t("proposal.kept"), draft.url);
    case "gone":
      return ended(draft, t("proposal.gone"), draft.channel_url);
    case "elsewhere":
      return ended(draft, t("proposal.elsewhere"), draft.url);
    default:
      return waiting(draft);
  }
}

// proposalHeader names where the draft waits, as a post's view names where
// it went.
function proposalHeader(draft) {
  const place = placeOf(draft);
  return el("header", { class: "thread-header" },
    el("span", { class: "muted posted-label" }, draft.root_id ? t("proposal.label_reply") : t("proposal.label")),
    el("h2", { class: "thread-title" },
      place.channel ? linkButton(place.channelURL, [place.channel], { class: "link channel-name" }) : t("thread.title")),
    place.team ? el("span", { class: "muted team" }, place.team) : null);
}

// preview is the text as it will be posted, or as written, with a warning
// when it holds characters the eye cannot see.
function preview(draft, text) {
  const place = placeOf(draft);
  const hidden = hiddenCharacters(text);
  return [
    el("div", { class: "proposal-text" },
      proposed.written ? asWritten(text) : renderMarkdown(text, { base: place.base, team: place.teamURL, targets: true })),
    hidden > 0 ? el("p", { class: "proposal-warning", role: "note" }, t("proposal.hidden", { count: hidden })) : null,
    el("button", {
      type: "button",
      class: "link muted written-toggle",
      "aria-pressed": String(proposed.written),
      onclick: () => {
        proposed.written = !proposed.written;
        render();
      },
    }, proposed.written ? t("proposal.as_posted") : t("proposal.as_written")),
  ];
}

// waiting is the proposal as it waits in the person's draft, with what they
// can do with it here.
function waiting(draft) {
  const changes = CHANGES && view.callsTools;
  const notes = Array.isArray(draft.notes) ? draft.notes : [];
  return el("article", { class: "thread proposal" },
    proposalHeader(draft),
    el("div", { class: "proposal-card" }, preview(draft, draft.message)),
    notes.length > 0 ? el("ul", { class: "proposal-notes muted" }, notes.map((note) => el("li", {}, note))) : null,
    el("p", { class: "muted proposal-status" }, POSTS && changes ? t("proposal.waits") : t("proposal.send_there")),
    proposed.confirm ? confirmation(draft, proposed.confirm) : null,
    el("div", { class: "proposal-actions" },
      POSTS && changes ? el("button", {
        type: "button",
        class: "action primary",
        disabled: proposed.busy || proposed.confirm || proposed.unsure ? true : undefined,
        onclick: () => press({ action: "post", shown: draft.message }),
      }, t("proposal.post")) : null,
      changes ? el("button", {
        type: "button",
        class: "action",
        disabled: proposed.busy ? true : undefined,
        onclick: () => {
          proposed.stage = "editing";
          proposed.text = draft.message;
          render();
        },
      }, t("proposal.edit")) : null,
      changes ? el("button", {
        type: "button",
        class: "action",
        disabled: proposed.busy ? true : undefined,
        onclick: () => press({ action: "discard", shown: draft.message }),
      }, t("proposal.discard")) : null,
      linkButton(draft.url, [t("proposal.open")], { class: "link muted" })));
}

// editing is the draft's text in a box the person changes, and saves into
// the draft.
function editing(draft) {
  const box = el("textarea", {
    class: "proposal-editor",
    rows: String(Math.min(12, Math.max(3, proposed.text.split("\n").length + 1))),
    "aria-label": t("proposal.edit"),
    oninput: (event) => {
      proposed.text = event.target.value;
    },
  }, proposed.text);
  return el("article", { class: "thread proposal" },
    proposalHeader(draft),
    box,
    el("div", { class: "proposal-actions" },
      el("button", {
        type: "button",
        class: "action primary",
        disabled: proposed.busy ? true : undefined,
        onclick: () => press({ action: "edit", shown: draft.message, message: proposed.text }),
      }, t("proposal.save")),
      el("button", {
        type: "button",
        class: "action",
        onclick: () => {
          proposed.stage = "proposed";
          render();
        },
      }, t("proposal.cancel"))));
}

// choosing is the person's own draft beside the proposal, or the proposal as
// a revision of it, for them to keep theirs or put the proposal in its place.
// Nothing is written until they press.
function choosing(draft, theirs) {
  const revises = Boolean(view.input && view.input.revises_draft);
  const place = placeOf(draft);
  const changes = CHANGES && view.callsTools;
  const differences = revises ? wordDifferences(theirs.message, draft.message) : null;
  return el("article", { class: "thread proposal" },
    proposalHeader(draft),
    el("p", { class: "muted proposal-status" }, t("proposal.choose")),
    differences
      ? el("section", { class: "proposal-card", "aria-label": t("proposal.revision") },
        el("h3", { class: "proposal-heading" }, t("proposal.revision")),
        el("pre", { class: "as-written diff" }, differences.map(([kind, text]) =>
          kind === "same" ? text : el(kind === "added" ? "ins" : "del", {}, text))))
      : [
        el("section", { class: "proposal-card", "aria-label": t("proposal.yours") },
          el("h3", { class: "proposal-heading" }, t("proposal.yours")),
          renderMarkdown(theirs.message, { base: place.base, team: place.teamURL, targets: true })),
        el("section", { class: "proposal-card", "aria-label": t("proposal.proposal") },
          el("h3", { class: "proposal-heading" }, t("proposal.proposal")),
          preview(draft, draft.message)),
      ],
    el("div", { class: "proposal-actions" },
      changes ? el("button", {
        type: "button",
        class: "action primary",
        disabled: proposed.busy ? true : undefined,
        onclick: () => press({ action: "replace", shown: theirs.message, message: draft.message }),
      }, revises ? t("proposal.apply") : t("proposal.replace")) : null,
      el("button", {
        type: "button",
        class: "action",
        onclick: () => {
          proposed.stage = "kept";
          tellModel("The person kept their own draft in " + placeName(draft) + "; the proposal was not saved.");
          render();
        },
      }, t("proposal.keep")),
      linkButton(theirs.url || draft.url, [t("proposal.open")], { class: "link muted" })));
}

// ended is what the view shows once the proposal is no longer to act on.
function ended(draft, text, url) {
  return el("article", { class: "thread proposal" },
    proposalHeader(draft),
    el("p", { class: "muted proposal-status", role: "status" }, text),
    el("div", { class: "proposal-actions" }, linkButton(url, [t("notice.open")], { class: "link muted" })));
}

// confirmation asks before a message that alerts a whole channel, or a user
// group, where Mattermost would ask.
function confirmation(draft, notifies) {
  const mentions = Array.isArray(notifies.mentions) ? notifies.mentions.map((mention) => String(mention).toLowerCase()) : [];
  const whole = mentions.some((mention) => WHOLE_CHANNEL.includes(mention));
  const here = mentions.length === 1 && mentions[0] === "@here";
  const people = Number(notifies.people) || 0;
  const zones = Number(notifies.timezones) || 0;
  let question;
  if (!whole) {
    question = t("confirm.group", { mention: notifies.mentions[0] || "", people });
  } else if (zones > 1) {
    question = here ? t("confirm.here_timezones", { people, count: zones }) : t("confirm.all_timezones", { people, count: zones });
  } else {
    question = here ? t("confirm.here", { people }) : t("confirm.all", { people });
  }
  const title = whole ? t("confirm.title") : t("confirm.title_groups");
  return el("div", { class: "confirm", role: "dialog", "aria-modal": "true", "aria-label": title },
    el("h3", { class: "confirm-title" }, title),
    el("p", { class: "confirm-question" }, question),
    el("div", { class: "proposal-actions" },
      el("button", {
        type: "button",
        class: "action",
        onclick: () => {
          proposed.confirm = null;
          render();
        },
      }, t("confirm.cancel")),
      el("button", {
        type: "button",
        class: "action primary",
        disabled: proposed.busy || proposed.unsure ? true : undefined,
        onclick: () => press({ action: "post", shown: draft.message, confirmed: true }),
      }, t("confirm.confirm"))));
}

// press sends what the person pressed through the tool only views call, for
// the draft the view shows, and draws what came of it. Where the host
// refuses the call, the view says so and offers the draft in Mattermost.
async function press(args) {
  if (proposed.busy) return;
  const draft = proposed.draft;
  proposed.busy = true;
  render();
  let result;
  try {
    const place = { channel_id: draft.channel_id };
    if (draft.root_id) place.root_id = draft.root_id;
    result = await bridge.callTool("view_change", Object.assign(place, args));
  } catch (error) {
    proposed.busy = false;
    // Where the host lost a Post, it may have gone through: the person looks
    // in Mattermost rather than posting it twice.
    if (args.action === "post" && error && error.unanswered) proposed.unsure = true;
    notice(proposed.unsure ? t("proposal.unsure") : t("proposal.refused"), draft.url);
    return;
  }
  proposed.busy = false;
  const change = result && !result.isError ? result.structuredContent : null;
  if (!change || typeof change.status !== "string") {
    notice((result && textOf(result)) || t("proposal.refused"), draft.url);
    return;
  }
  outcome(args, change);
}

// outcome draws what came of a press, and tells the model what the person
// did, in mm-mcp's words.
function outcome(args, change) {
  const draft = proposed.draft;
  proposed.confirm = null;
  switch (change.status) {
    case "posted":
      proposed.stage = "posted";
      proposed.post = change.post;
      tellModel("The person posted the proposal from its view, in " + placeName(draft) + ": " + (change.post && change.post.url ? change.post.url : "") +
        ". The text posted, as a JSON string: " + JSON.stringify(change.post ? change.post.message : draft.message));
      break;
    case "confirm":
      proposed.confirm = change.notifies || null;
      break;
    case "saved":
      proposed.draft = Object.assign({}, draft, change.draft);
      proposed.stage = "proposed";
      if (args.action === "replace") {
        proposed.theirs = null;
        tellModel("The person put the proposal in place of their own draft in " + placeName(draft) + "; it waits there for them to send.");
      } else {
        notice(t("proposal.edited"));
        tellModel("The person changed the proposal's text in its view. The draft in " + placeName(draft) +
          " is theirs now, and propose_post leaves it as it is. Its text, as a JSON string: " + JSON.stringify(change.draft ? change.draft.message : args.message));
      }
      break;
    case "discarded":
      proposed.stage = "discarded";
      tellModel("The person discarded the proposal in " + placeName(draft) + ".");
      break;
    case "changed":
      // Drawn again as it is now: what the person sees is what a press acts on.
      if (proposed.stage === "choosing") {
        proposed.theirs = Object.assign({}, proposed.theirs, change.draft);
      } else {
        proposed.draft = Object.assign({}, draft, change.draft);
        proposed.stage = "proposed";
      }
      notice(t("proposal.changed"));
      break;
    case "elsewhere":
      proposed.stage = "elsewhere";
      break;
    case "gone":
      proposed.stage = "gone";
      break;
  }
  render();
}

// tellModel puts what the person did in the context the model reads next,
// where the host passes it on; the text it quotes stays data.
function tellModel(text) {
  if (!view.tellsModel) return;
  bridge.updateModelContext(text).catch(() => {});
}

// placeName names the draft's place for the model, as data.
function placeName(draft) {
  return "the channel " + JSON.stringify(draft.channel || draft.channel_id || "");
}

// HIDDEN is each character a reader cannot see, or that turns the text's
// direction, which a message could hide what it says behind: what Unicode
// says a reader's software may draw as nothing, such as soft hyphens,
// zero-width spaces and joiners, fillers, variation selectors and tag
// characters, and the marks and overrides that set a text's direction.
const HIDDEN = /[\p{Default_Ignorable_Code_Point}\p{Bidi_Control}]/u;
const PICTOGRAPH = /\p{Extended_Pictographic}/u;
const MODIFIER = /\p{Emoji_Modifier}/u;
const EMOJI = /\p{Emoji}/u;
const IDEOGRAPH = /\p{Ideographic}/u;
const VARIATION = /\p{Variation_Selector}/u;

// NUMERIC_REFERENCE is a character written as its number, which Markdown
// draws as the character itself.
const NUMERIC_REFERENCE = /^&#(?:[xX]([0-9a-fA-F]{1,6})|([0-9]{1,7}));/;

// hiddenAt reports whether the character at i of chars is hidden. What an
// emoji or a name is drawn with is what it looks like: a zero-width joiner
// between two pictographs, as in a family's emoji; one variation selector,
// after an emoji or an ideograph, which picks how it is drawn; and the tags
// after a black flag that make it Scotland's. A second selector in a row, or
// tags anywhere else, carry what no reader sees.
function hiddenAt(chars, i) {
  const ch = chars[i];
  if (!HIDDEN.test(ch)) return false;
  const code = ch.codePointAt(0);
  const before = chars[i - 1];
  if (code === 0x200d) {
    const joined = before && VARIATION.test(before) ? chars[i - 2] : before;
    const after = chars[i + 1];
    return !(joined && (PICTOGRAPH.test(joined) || MODIFIER.test(joined)) && after && PICTOGRAPH.test(after));
  }
  if (VARIATION.test(ch)) return !(before && !VARIATION.test(before) && (EMOJI.test(before) || IDEOGRAPH.test(before)));
  if (code >= 0xe0020 && code <= 0xe007f) return !inSubdivisionFlag(chars, i);
  return true;
}

// inSubdivisionFlag reports whether the tag at i of chars is in a flag of a
// country's part: a black flag, up to six tags, and the tag that ends them.
function inSubdivisionFlag(chars, i) {
  const at = (k) => (chars[k] ? chars[k].codePointAt(0) : -1);
  const tag = (k) => at(k) >= 0xe0020 && at(k) <= 0xe007e;
  let start = i;
  while (start > i - 7 && tag(start - 1)) start--;
  let end = i;
  while (end < i + 7 && tag(end)) end++;
  return at(start - 1) === 0x1f3f4 && end - start >= 1 && end - start <= 6 && at(end) === 0xe007f;
}

// hiddenParts is text as what a reader sees and what they cannot: runs of
// text, and each hidden character, by its code point, with how it is written
// when it is written as its number.
function hiddenParts(text) {
  const chars = Array.from(String(text || ""));
  const parts = [];
  let run = "";
  const hide = (code, written) => {
    parts.push(run, { code: "U+" + code.toString(16).toUpperCase().padStart(4, "0"), written });
    run = "";
  };
  for (let i = 0; i < chars.length; i++) {
    if (chars[i] === "&") {
      const reference = NUMERIC_REFERENCE.exec(chars.slice(i, i + 12).join(""));
      const code = reference ? parseInt(reference[1] || reference[2], reference[1] ? 16 : 10) : 0;
      if (code > 0 && code <= 0x10ffff && (code < 0xd800 || code > 0xdfff) && HIDDEN.test(String.fromCodePoint(code))) {
        hide(code, reference[0]);
        i += reference[0].length - 1;
        continue;
      }
    }
    if (hiddenAt(chars, i)) {
      hide(chars[i].codePointAt(0), "");
      continue;
    }
    run += chars[i];
  }
  parts.push(run);
  return parts.filter((part) => part !== "");
}

function hiddenCharacters(text) {
  return hiddenParts(text).filter((part) => typeof part !== "string").length;
}

// asWritten is the text as it was written, each hidden character shown by its
// code point, and one written as its number marked where it stands.
function asWritten(text) {
  return el("pre", { class: "as-written" }, hiddenParts(text).map((part) => typeof part === "string"
    ? part
    : el("span", { class: "hidden-char", title: part.code }, part.written || part.code)));
}

// MAX_DIFF_CELLS bounds the table a revision's differences are found with; a
// longer pair of texts is shown side by side instead.
const MAX_DIFF_CELLS = 250000;

// wordDifferences is how after differs from before, word by word: runs of
// words kept, removed and added, in order, or null for texts too long to
// compare.
function wordDifferences(before, after) {
  const a = String(before || "").split(/(\s+)/).filter((word) => word !== "");
  const b = String(after || "").split(/(\s+)/).filter((word) => word !== "");
  if ((a.length + 1) * (b.length + 1) > MAX_DIFF_CELLS) return null;
  const width = b.length + 1;
  const common = new Uint32Array((a.length + 1) * width);
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      common[i * width + j] = a[i] === b[j] ? common[(i + 1) * width + j + 1] + 1 : Math.max(common[(i + 1) * width + j], common[i * width + j + 1]);
    }
  }
  const parts = [];
  const add = (kind, word) => {
    const last = parts[parts.length - 1];
    if (last && last[0] === kind) last[1] += word;
    else parts.push([kind, word]);
  };
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      add("same", a[i]);
      i++;
      j++;
    } else if (common[(i + 1) * width + j] >= common[i * width + j + 1]) {
      add("removed", a[i++]);
    } else {
      add("added", b[j++]);
    }
  }
  while (i < a.length) add("removed", a[i++]);
  while (j < b.length) add("added", b[j++]);
  return parts;
}
