// A message, drawn from its Markdown as Mattermost draws it: headings,
// paragraphs, lists and task lists, quotes, fenced and indented code, tables,
// rules, emphasis, strikethrough, code spans and links, and Mattermost's own
// @mentions, ~channel references and :emoji:. As in Mattermost, a single line
// break is a break. Ported from bitbucket-data-center-cli's parser.
//
// The parser builds elements with el() and text nodes, and never hands a
// string to the browser to parse, so HTML in a message stays text, as
// everything else a view draws does. A link opens through the host, and only
// when it leads to a web page. An image in a message is named rather than
// drawn: a view has no network.

// MD_MAX_DEPTH is how deeply quotes, lists and emphasis nest before what is
// inside is drawn as text.
const MD_MAX_DEPTH = 8;

// MD_MAX_SCAN is how far an opening mark looks for its close, which keeps a
// description built to be slow, such as thousands of unclosed marks, fast.
const MD_MAX_SCAN = 2000;

// MD_MAX_WORK is how many characters, for each character of a run of text,
// its marks look through together for their closes. A message built to be
// slow draws its last marks as text rather than freezing the view.
const MD_MAX_WORK = 32;

const MD_FENCE = /^( {0,3})(`{3,}(?=[^`]*$)|~{3,})(.*)$/;
const MD_SETEXT = /^ {0,3}(=+|-+)[ \t]*$/;
const MD_RULE = /^ {0,3}(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})$/;
const MD_QUOTE = /^ {0,3}>/;
const MD_ITEM = /^( {0,3})([-+*]|\d{1,9}[.)])(?:([ \t]+)(.*))?$/;
const MD_TABLE_DELIMITER = /^ {0,3}\|?[ \t]*:?-+:?[ \t]*(?:\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$/;
const MD_REFERENCE = /^ {0,3}\[([^\]]+)\]:[ \t]*<?([^\s>]+)>?(?:[ \t]+(?:"([^"]*)"|'([^']*)'|\(([^)]*)\)))?[ \t]*$/;
const MD_PUNCTUATION = /[!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~]/;
const MD_WORD = /[\p{L}\p{N}]/u;

const MD_ENTITIES = {
  amp: "&", lt: "<", gt: ">", quot: "\"", apos: "'", nbsp: " ", copy: "©", reg: "®", trade: "™",
  hellip: "…", mdash: "—", ndash: "–", lsquo: "‘", rsquo: "’", ldquo: "“", rdquo: "”", times: "×", middot: "·", bull: "•",
};

// renderMarkdown draws source. options.base is the address a link starting
// with "/" is taken against, the Mattermost the message is on, and
// options.team the address of its team, which a ~channel reference opens in.
// options.targets shows beside each link where it leads, when its text says
// otherwise, as a proposal's preview does.
function renderMarkdown(source, options) {
  // A line ends wherever a browser breaks one, so no pattern below meets a
  // line break it cannot match across.
  const lines = String(source || "")
    .split(/\r\n?|\n|\u2028|\u2029/)
    .map((line) => line.replace(/^\t+/, (tabs) => "    ".repeat(tabs.length)));
  const { references, definitions } = markdownReferences(lines);
  const context = { base: options.base, team: options.team, targets: Boolean(options.targets), references };
  // Mattermost draws no definition. A proposal's preview shows each where it
  // stands, since the post carries it all the same.
  const blocks = [];
  let from = 0;
  for (const at of definitions) {
    blocks.push(...markdownBlocks(lines.slice(from, at), context, 0));
    if (context.targets) blocks.push(el("p", { class: "md-definition" }, lines[at].trim()));
    from = at + 1;
  }
  blocks.push(...markdownBlocks(lines.slice(from), context, 0));
  return el("div", { class: "markdown" }, blocks);
}

// markdownReferences finds the link definitions outside code, so a link can
// name one defined further down: references, by label, and definitions, the
// lines they stand on. As in CommonMark, a definition does not interrupt a
// paragraph; a line in one that reads like a definition is its text.
function markdownReferences(lines) {
  const references = new Map();
  const definitions = [];
  let fence = null;
  let paragraph = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const opening = MD_FENCE.exec(line);
    if (fence) {
      if (opening && opening[2][0] === fence[0] && opening[2].length >= fence.length && opening[3].trim() === "") fence = null;
      continue;
    }
    if (opening) {
      fence = opening[2];
      continue;
    }
    const definition = paragraph ? null : MD_REFERENCE.exec(line);
    if (definition) {
      const label = normalizeLabel(definition[1]);
      if (!references.has(label)) {
        references.set(label, { url: definition[2], title: definition[3] || definition[4] || definition[5] || "" });
      }
      definitions.push(i);
      continue;
    }
    paragraph = !isBlank(line) && !atxHeading(line) && !MD_RULE.test(line);
  }
  return { references, definitions };
}

function normalizeLabel(label) {
  return label.trim().replace(/\s+/g, " ").toLowerCase();
}

function isBlank(line) {
  return line.trim() === "";
}

function markdownBlocks(lines, context, depth) {
  if (depth > MD_MAX_DEPTH) return [el("p", {}, lines.join("\n"))];
  const blocks = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (isBlank(line)) {
      i++;
      continue;
    }
    let block;
    let match;
    if ((match = MD_FENCE.exec(line))) {
      [block, i] = fencedCode(lines, i, match);
    } else if ((match = atxHeading(line))) {
      block = markdownHeading(match.level, match.text, context);
      i++;
    } else if (MD_RULE.test(line)) {
      block = el("hr", { class: "md-rule" });
      i++;
    } else if (MD_QUOTE.test(line)) {
      [block, i] = blockQuote(lines, i, context, depth);
    } else if (MD_ITEM.test(line)) {
      [block, i] = markdownList(lines, i, context, depth);
    } else if (/^ {4}/.test(line)) {
      [block, i] = indentedCode(lines, i);
    } else if (startsTable(lines, i)) {
      [block, i] = markdownTable(lines, i, context);
    } else {
      [block, i] = markdownParagraph(lines, i, context);
    }
    blocks.push(block);
  }
  return blocks;
}

// atxHeading reads a heading written with one to six #, or nothing when the
// line is not one. It reads by hand: as a pattern, its optional parts take
// time that grows with the cube of a run of spaces.
function atxHeading(line) {
  const blank = (k) => line[k] === " " || line[k] === "\t";
  let i = 0;
  while (i < 3 && line[i] === " ") i++;
  let level = 0;
  while (line[i + level] === "#") level++;
  if (level < 1 || level > 6) return null;
  i += level;
  if (i < line.length && !blank(i)) return null;
  let end = line.length;
  while (end > i && blank(end - 1)) end--;
  // A closing run of #, after a space or as all there is, is not text.
  let close = end;
  while (close > i && line[close - 1] === "#") close--;
  if (close < end && (close === i || blank(close - 1))) end = close;
  while (i < end && blank(i)) i++;
  while (end > i && blank(end - 1)) end--;
  return { level, text: line.slice(i, end) };
}

function markdownHeading(level, text, context) {
  // The view's own headings come first, so a message's sit below them.
  return el("h" + Math.min(level + 2, 6), { class: "md-h md-h" + level }, markdownInline(text, context, 0, false));
}

function fencedCode(lines, start, opening) {
  const indent = opening[1].length;
  const fence = opening[2];
  const content = [];
  let i = start + 1;
  for (; i < lines.length; i++) {
    const closing = MD_FENCE.exec(lines[i]);
    if (closing && closing[2][0] === fence[0] && closing[2].length >= fence.length && closing[3].trim() === "") {
      i++;
      break;
    }
    content.push(lines[i].replace(new RegExp("^ {0," + indent + "}"), ""));
  }
  const language = opening[3].trim().split(/\s+/)[0];
  return [el("pre", { class: "md-code", "data-language": language || undefined }, el("code", {}, content.join("\n"))), i];
}

function indentedCode(lines, start) {
  const content = [];
  let i = start;
  for (; i < lines.length; i++) {
    if (/^ {4}/.test(lines[i])) content.push(lines[i].slice(4));
    else if (isBlank(lines[i])) content.push("");
    else break;
  }
  while (content.length > 0 && content[content.length - 1] === "") content.pop();
  return [el("pre", { class: "md-code" }, el("code", {}, content.join("\n"))), i];
}

// interrupts reports whether a line starts a block that ends a paragraph.
function interrupts(lines, i) {
  const line = lines[i];
  if (MD_FENCE.test(line) || atxHeading(line) || MD_RULE.test(line) || MD_QUOTE.test(line) || startsTable(lines, i)) return true;
  const item = MD_ITEM.exec(line);
  // A list interrupts a paragraph when it has content, and an ordered one
  // only when it starts at one.
  return Boolean(item && item[4] && item[4].trim() !== "" && (!/\d/.test(item[2]) || parseInt(item[2], 10) === 1));
}

function markdownParagraph(lines, start, context) {
  const text = [];
  let i = start;
  while (i < lines.length && !isBlank(lines[i])) {
    if (i > start) {
      const underline = MD_SETEXT.exec(lines[i]);
      if (underline) return [markdownHeading(underline[1][0] === "=" ? 1 : 2, text.join("\n"), context), i + 1];
      if (interrupts(lines, i)) break;
    }
    text.push(lines[i].trim());
    i++;
  }
  return [el("p", {}, markdownInline(text.join("\n"), context, 0, false)), i];
}

function blockQuote(lines, start, context, depth) {
  const inner = [];
  let i = start;
  while (i < lines.length) {
    const line = lines[i];
    if (MD_QUOTE.test(line)) {
      inner.push(line.replace(/^ {0,3}> ?/, ""));
      i++;
    } else if (!isBlank(line) && inner.length > 0 && !isBlank(inner[inner.length - 1]) && !interrupts(lines, i)) {
      // A line that carries on the quote's paragraph without its own marker.
      inner.push(line);
      i++;
    } else {
      break;
    }
  }
  return [el("blockquote", { class: "md-quote" }, markdownBlocks(inner, context, depth + 1)), i];
}

function markdownList(lines, start, context, depth) {
  const first = MD_ITEM.exec(lines[start]);
  const ordered = /\d/.test(first[2]);
  const marker = ordered ? first[2].slice(-1) : first[2];
  const items = [];
  let i = start;
  while (i < lines.length) {
    const match = MD_ITEM.exec(lines[i]);
    if (!match || /\d/.test(match[2]) !== ordered || (ordered ? match[2].slice(-1) : match[2]) !== marker) break;
    const spacing = match[3] ? match[3].length : 1;
    // Content indented five or more past the marker is indented code, which
    // starts one space in.
    const contentIndent = match[1].length + match[2].length + (spacing > 4 ? 1 : spacing);
    const content = [spacing > 4 ? " ".repeat(spacing - 1) + (match[4] || "") : match[4] || ""];
    i++;
    let blank = false;
    while (i < lines.length) {
      const line = lines[i];
      if (isBlank(line)) {
        content.push("");
        blank = true;
        i++;
        continue;
      }
      if (line.match(/^ */)[0].length >= contentIndent) {
        content.push(line.slice(contentIndent));
        blank = false;
        i++;
        continue;
      }
      // After a blank line, an unindented line ends the item; before one, a
      // line of text carries on its paragraph.
      if (blank || MD_ITEM.test(line) || interrupts(lines, i)) break;
      content.push(line.trim());
      i++;
    }
    while (content.length > 0 && content[content.length - 1] === "") content.pop();
    items.push({ content, number: ordered ? parseInt(match[2], 10) : null });
  }

  const drawn = items.map((item) => {
    const task = /^\[([ xX])\][ \t]+/.exec(item.content[0] || "");
    if (!task) return el("li", {}, markdownBlocks(item.content, context, depth + 1));
    const rest = [item.content[0].slice(task[0].length), ...item.content.slice(1)];
    return el("li", { class: "md-task" },
      el("input", { type: "checkbox", disabled: true, checked: task[1] !== " ", "aria-label": task[1] !== " " ? t("markdown.task_done") : t("markdown.task_open") }),
      markdownBlocks(rest, context, depth + 1));
  });
  const start1 = items.length > 0 ? items[0].number : null;
  return [ordered
    ? el("ol", { class: "md-list", start: start1 !== null && start1 !== 1 ? start1 : undefined }, drawn)
    : el("ul", { class: "md-list" }, drawn), i];
}

function startsTable(lines, i) {
  return i + 1 < lines.length && lines[i].includes("|") && MD_TABLE_DELIMITER.test(lines[i + 1]) &&
    tableCells(lines[i]).length === tableCells(lines[i + 1]).length;
}

function tableCells(line) {
  let text = line.trim();
  if (text.startsWith("|")) text = text.slice(1);
  if (text.endsWith("|") && !text.endsWith("\\|")) text = text.slice(0, -1);
  const cells = [];
  let cell = "";
  for (let k = 0; k < text.length; k++) {
    if (text[k] === "\\" && text[k + 1] === "|") {
      cell += "|";
      k++;
    } else if (text[k] === "|") {
      cells.push(cell.trim());
      cell = "";
    } else {
      cell += text[k];
    }
  }
  cells.push(cell.trim());
  return cells;
}

function markdownTable(lines, start, context) {
  const header = tableCells(lines[start]);
  const aligns = tableCells(lines[start + 1]).map((cell) =>
    cell.startsWith(":") && cell.endsWith(":") ? "center" : cell.endsWith(":") ? "right" : cell.startsWith(":") ? "left" : "");
  const rows = [];
  let i = start + 2;
  while (i < lines.length && !isBlank(lines[i]) && lines[i].includes("|") && !MD_FENCE.test(lines[i]) && !MD_QUOTE.test(lines[i])) {
    rows.push(tableCells(lines[i]));
    i++;
  }
  const cell = (tag, text, column) => el(tag, { class: aligns[column] ? "align-" + aligns[column] : undefined },
    markdownInline(text, context, 0, false));
  // A wide table scrolls sideways inside its own box, never the view.
  return [el("div", { class: "md-table-wrap" }, el("table", { class: "md-table" },
    el("thead", {}, el("tr", {}, header.map((text, column) => cell("th", text, column)))),
    rows.length > 0 ? el("tbody", {}, rows.map((row) => el("tr", {}, header.map((_, column) => cell("td", row[column] || "", column))))) : null)), i];
}

// markdownInline draws a run of text: code spans, links, emphasis, line
// breaks and escapes. Inside a link's text, links are text.
function markdownInline(text, context, depth, inLink) {
  const out = [];
  const marks = inlineMarks(text);
  let buffer = "";
  const flush = () => {
    if (buffer) out.push(buffer);
    buffer = "";
  };
  let i = 0;
  while (i < text.length) {
    const ch = text[i];
    const next = text[i + 1];
    let found = null;
    if (ch === "\\" && next === "\n") {
      found = { node: el("br"), end: i + 2 };
    } else if (ch === "\\" && next !== undefined && MD_PUNCTUATION.test(next)) {
      buffer += next;
      i += 2;
      continue;
    } else if (ch === "\n") {
      found = { node: el("br"), end: i + 1 };
    } else if (ch === "&") {
      const entity = /^&(#\d{1,7}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});/.exec(text.slice(i, i + 40));
      const decoded = entity ? decodeEntity(entity[1]) : null;
      if (decoded !== null) {
        buffer += decoded;
        i += entity[0].length;
        continue;
      }
    } else if (ch === "`") {
      found = codeSpan(text, i, marks);
    } else if (ch === "<") {
      found = inLink ? null : autolink(text, i, context);
    } else if (ch === "!" && next === "[") {
      found = inLink ? null : linkOrImage(text, i + 1, context, depth, true, marks);
    } else if (ch === "[") {
      found = inLink ? null : linkOrImage(text, i, context, depth, false, marks);
    } else if (ch === "*" || ch === "_" || (ch === "~" && next === "~")) {
      found = emphasis(text, i, context, depth, inLink, marks);
    } else if ((ch === "h" || ch === "H") && !inLink && (i === 0 || /[\s(]/.test(text[i - 1])) && startsWebAddress(text, i)) {
      found = bareLink(text, i, context, marks);
    } else if (ch === "@" && (i === 0 || !MD_NAME_CHAR.test(text[i - 1]))) {
      found = mention(text, i);
    } else if (ch === "~" && next !== "~" && !inLink && (i === 0 || !MD_NAME_CHAR.test(text[i - 1]))) {
      found = channelReference(text, i, context);
    } else if (ch === ":" && (i === 0 || !MD_NAME_CHAR.test(text[i - 1]))) {
      found = emojiName(text, i);
    }
    if (found) {
      flush();
      out.push(found.node);
      i = found.end;
      continue;
    }
    buffer += ch;
    i++;
  }
  flush();
  return out;
}

function decodeEntity(name) {
  if (name[0] === "#") {
    const code = name[1] === "x" || name[1] === "X" ? parseInt(name.slice(2), 16) : parseInt(name.slice(1), 10);
    if (!Number.isFinite(code) || code <= 0 || code > 0x10ffff || (code >= 0xd800 && code <= 0xdfff)) return "�";
    return String.fromCodePoint(code);
  }
  return Object.prototype.hasOwnProperty.call(MD_ENTITIES, name) ? MD_ENTITIES[name] : null;
}

// inlineMarks is what the marks in a run of text share as they look for
// their closes: where its runs of backticks are, by length, the brackets
// paired so far, and the work left, which keeps the time a message takes in
// step with its length, however it is built.
function inlineMarks(text) {
  const runs = new Map();
  const lengths = new Map();
  for (let k = text.indexOf("`"); k >= 0; k = text.indexOf("`", k)) {
    let run = 1;
    while (text[k + run] === "`") run++;
    if (!runs.has(run)) runs.set(run, []);
    runs.get(run).push(k);
    lengths.set(k, run);
    k += run;
  }
  return { runs, lengths, brackets: new Map(), work: MD_MAX_WORK * text.length + MD_MAX_SCAN * 4 };
}

// codeClose finds what closes the run of backticks at text[start]: the next
// run as long, whose start is close, or nothing, when close is -1.
function codeClose(text, start, marks) {
  let run = marks.lengths.get(start);
  if (run === undefined) {
    // A run whose first backtick was escaped.
    run = 0;
    while (text[start + run] === "`") run++;
    marks.work -= run;
  }
  const starts = marks.runs.get(run) || [];
  let low = 0;
  let high = starts.length;
  while (low < high) {
    const middle = (low + high) >> 1;
    if (starts[middle] < start + run) low = middle + 1;
    else high = middle;
  }
  return { run, close: low < starts.length ? starts[low] : -1 };
}

// codeEnd is where the code span opening at text[start] ends, or where its
// backticks end when nothing closes them.
function codeEnd(text, start, marks) {
  const { run, close } = codeClose(text, start, marks);
  return close < 0 ? start + run : close + run;
}

// codeSpan is the code between a run of backticks and the next run as long,
// or the backticks themselves when nothing closes them.
function codeSpan(text, start, marks) {
  const { run, close } = codeClose(text, start, marks);
  if (close < 0) return { node: "`".repeat(run), end: start + run };
  let code = text.slice(start + run, close).replace(/\n/g, " ");
  if (code.length > 2 && code.startsWith(" ") && code.endsWith(" ") && code.trim() !== "") code = code.slice(1, -1);
  return { node: el("code", { class: "md-code-span" }, code), end: close + run };
}

function autolink(text, start, context) {
  const match = /^<([a-zA-Z][a-zA-Z0-9+.-]{1,31}:[^\s<>]*)>/.exec(text.slice(start, start + 2048));
  if (!match) return null;
  return { node: markdownLink([match[1]], match[1], "", context), end: start + match[0].length };
}

// MD_NAME_CHAR is a character of a username, a channel's name or an emoji's,
// which a mention, a reference or an emoji may not follow.
const MD_NAME_CHAR = /[A-Za-z0-9_.+-]/;
const MD_MENTION = /^@([A-Za-z0-9][A-Za-z0-9._-]{0,63})/;
const MD_CHANNEL = /^~([a-z0-9][a-z0-9_-]{0,63})/;
const MD_EMOJI = /^:([a-zA-Z0-9_+-]{1,64}):/;

// mention is an @mention as Mattermost marks one, less the full stops, dashes
// and underscores that end the sentence around it.
function mention(text, start) {
  const match = MD_MENTION.exec(text.slice(start, start + 80));
  if (!match) return null;
  const name = match[1].replace(/[._-]+$/, "");
  if (!name) return null;
  const everyone = ["all", "channel", "here"].includes(name.toLowerCase());
  return { node: el("span", { class: everyone ? "md-mention md-mention-all" : "md-mention" }, "@" + name), end: start + 1 + name.length };
}

// channelReference is a ~channel reference, which opens the channel in its
// team when the view knows the team's address.
function channelReference(text, start, context) {
  const match = MD_CHANNEL.exec(text.slice(start, start + 80));
  if (!match) return null;
  const name = match[1].replace(/[_-]+$/, "");
  const label = "~" + name;
  const target = context.team ? webAddress(context.team + "/channels/" + name, "") : "";
  const node = target
    ? el("button", { type: "button", class: "md-link md-channel", title: target, onclick: () => openLink(target) }, label)
    : el("span", { class: "md-channel" }, label);
  return { node, end: start + label.length };
}

// emojiName is an :emoji: by its name: one that comes with Mattermost as its
// character, a custom one as its picture once it arrives, and any other as
// the text it is.
function emojiName(text, start) {
  const match = MD_EMOJI.exec(text.slice(start, start + 70));
  if (!match) return null;
  return { node: emoji(match[1]), end: start + match[0].length };
}

// startsWebAddress reports whether a web address starts at text[i].
function startsWebAddress(text, i) {
  const head = text.slice(i, i + 8).toLowerCase();
  return head.startsWith("https://") || head.startsWith("http://");
}

// bareLink is a web address written out in the text, as GitHub and Bitbucket
// link one, less the punctuation that ends the sentence around it.
function bareLink(text, start, context, marks) {
  if (marks.work <= 0) return null;
  let end = start;
  while (end < text.length && end - start < 2048 && !/[\s<]/.test(text[end])) end++;
  marks.work -= end - start;
  while (end > start && /[.,:;!?"'*_~]/.test(text[end - 1])) end--;
  const url = text.slice(start, end);
  if (url.endsWith(")") && (url.match(/\)/g) || []).length > (url.match(/\(/g) || []).length) end--;
  const address = text.slice(start, end);
  if (!webAddress(address, "")) return null;
  return { node: markdownLink([address], address, "", context), end };
}

// linkOrImage reads a link or an image whose text opens at text[open]: an
// inline one, [text](url "title"), or one that names a definition.
function linkOrImage(text, open, context, depth, image, marks) {
  const close = closingBracket(text, open, marks);
  if (close < 0) return null;
  const label = text.slice(open + 1, close);
  let url;
  let title = "";
  let end;
  if (text[close + 1] === "(") {
    const destination = linkDestination(text, close + 2, marks);
    if (!destination) return null;
    ({ url, title, end } = destination);
  } else {
    let name = label;
    end = close + 1;
    if (text[close + 1] === "[") {
      const nameClose = text.indexOf("]", close + 2);
      if (nameClose > 0 && nameClose - close < 1000) {
        name = text.slice(close + 2, nameClose) || label;
        end = nameClose + 1;
      }
    }
    const definition = context.references.get(normalizeLabel(name));
    if (!definition) return null;
    ({ url, title } = definition);
  }
  const node = image
    ? markdownImage(label, url, context)
    : markdownLink(markdownInline(label, context, depth + 1, true), url, title, context);
  return { node, end };
}

// closingBracket is where the [ at text[open] closes, or -1 when nothing
// does. One pass pairs it and every [ inside it, so a later [ that pass met
// is answered without another.
function closingBracket(text, open, marks) {
  if (marks.brackets.has(open)) return marks.brackets.get(open);
  const opened = [];
  for (let k = open; k < text.length; k++) {
    if (--marks.work < 0) return -1;
    const ch = text[k];
    if (ch === "\\") {
      k++;
    } else if (ch === "`") {
      // A run of backticks is passed whole, whether or not it closes.
      k = codeEnd(text, k, marks) - 1;
    } else if (ch === "[") {
      opened.push(k);
    } else if (ch === "]") {
      const at = opened.pop();
      marks.brackets.set(at, k);
      if (opened.length === 0) return k;
    }
  }
  for (const at of opened) marks.brackets.set(at, -1);
  return -1;
}

function linkDestination(text, start, marks) {
  if (marks.work <= 0) return null;
  let i = start;
  while (text[i] === " " || text[i] === "\n") i++;
  let url = "";
  if (text[i] === "<") {
    const close = text.indexOf(">", i + 1);
    marks.work -= (close < 0 ? text.length : close) - start;
    if (close < 0 || text.slice(i + 1, close).includes("\n")) return null;
    url = text.slice(i + 1, close);
    i = close + 1;
  } else {
    let level = 0;
    const from = i;
    for (; i < text.length && i - from < 4096; i++) {
      const ch = text[i];
      if (ch === "\\" && MD_PUNCTUATION.test(text[i + 1] || "")) {
        i++;
      } else if (/\s/.test(ch)) {
        break;
      } else if (ch === "(") {
        level++;
      } else if (ch === ")") {
        if (level === 0) break;
        level--;
      }
    }
    marks.work -= i - start;
    url = text.slice(from, i).replace(/\\([!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~])/g, "$1");
  }
  while (text[i] === " " || text[i] === "\n") i++;
  let title = "";
  const quote = text[i];
  if (quote === "\"" || quote === "'" || quote === "(") {
    const closer = quote === "(" ? ")" : quote;
    const close = text.indexOf(closer, i + 1);
    marks.work -= (close < 0 ? text.length : close) - i;
    if (close < 0 || marks.work < 0) return null;
    title = text.slice(i + 1, close);
    i = close + 1;
    while (text[i] === " " || text[i] === "\n") i++;
  }
  if (text[i] !== ")") return null;
  return { url, title, end: i + 1 };
}

// webAddress is where a link leads, when that is a web page: an absolute
// http or https address, a path on the Mattermost the message is on, or, as
// Mattermost reads them, an address with no scheme, which is a web page's,
// and one starting with //, which is on the Mattermost's own scheme.
function webAddress(url, base) {
  let target = String(url || "").trim();
  const web = /^https?:/i.test(base || "") ? base : "https://localhost";
  if (target && !target.startsWith("/") && !/^[a-z0-9+.-]+:/i.test(target)) target = "http://" + target;
  try {
    const parsed = target.startsWith("//") ? new URL(target, web)
      : target.startsWith("/") && base ? new URL(target, base) : new URL(target);
    return parsed.protocol === "https:" || parsed.protocol === "http:" ? parsed.href : "";
  } catch {
    return "";
  }
}

function markdownLink(children, url, title, context) {
  const target = webAddress(url, context.base);
  if (!target) {
    const text = el("span", { class: "md-link-text" }, children);
    // Mattermost may still make it a link, to an app or a mail address, so a
    // proposal's preview says where, though the view opens only web pages.
    const written = String(url || "").trim();
    return context.targets && written ? el("span", { class: "md-link-shown" }, text, linkTarget(text.textContent, written)) : text;
  }
  const link = el("button", {
    type: "button",
    class: "md-link",
    title: title ? title + " (" + target + ")" : target,
    onclick: () => openLink(target),
  }, children);
  return context.targets ? el("span", { class: "md-link-shown" }, link, linkTarget(link.textContent, target)) : link;
}

// linkTarget shows where a link leads, when its text does not say it.
function linkTarget(text, target) {
  const said = text.trim();
  if (said === target || said + "/" === target) return null;
  return el("span", { class: "md-link-target" }, target);
}

function markdownImage(alt, url, context) {
  const target = webAddress(url, context.base);
  const label = [icon("image"), alt || t("markdown.image")];
  if (!target) {
    const image = el("span", { class: "md-image", title: t("markdown.image_unseen") }, label);
    const written = String(url || "").trim();
    return context.targets && written ? el("span", { class: "md-link-shown" }, image, linkTarget("", written)) : image;
  }
  const image = el("button", { type: "button", class: "md-link md-image", title: t("markdown.image_open", { url: target }), onclick: () => openLink(target) }, label);
  return context.targets ? el("span", { class: "md-link-shown" }, image, linkTarget("", target)) : image;
}

// emphasis reads emphasis, strong emphasis or strikethrough opening at
// text[start], or its run of marks as text when it does not open or nothing
// closes it. An underscore inside a word, as in snake_case, is a letter.
function emphasis(text, start, context, depth, inLink, marks) {
  if (depth >= MD_MAX_DEPTH) return null;
  const ch = text[start];
  let run = 0;
  while (text[start + run] === ch) run++;
  // The whole run, so its later marks are not read again one by one.
  const asText = { node: text.slice(start, start + run), end: start + run };
  if (ch === "~" && run !== 2) return asText;
  const before = start > 0 ? text[start - 1] : " ";
  const after = text[start + run] || " ";
  if (/\s/.test(after) || (ch === "_" && MD_WORD.test(before))) return asText;
  const size = ch === "~" ? 2 : Math.min(run, 3);
  const open = start + run - size;

  const limit = Math.min(text.length, open + size + MD_MAX_SCAN);
  for (let k = open + size; k < limit;) {
    if (--marks.work < 0) return asText;
    const c = text[k];
    if (c === "\\") {
      k += 2;
      continue;
    }
    if (c === "`") {
      k = codeEnd(text, k, marks);
      continue;
    }
    if (c !== ch) {
      k++;
      continue;
    }
    let close = 0;
    while (text[k + close] === ch) close++;
    marks.work -= close;
    const previous = text[k - 1];
    const following = text[k + close] || " ";
    const canClose = k > open + size && !/\s/.test(previous) && (ch !== "_" || !MD_WORD.test(following));
    // A closing run matches an opening run of its own size, or closes one
    // inside it as well when it is longer: ***both*** and **strong *em***.
    if (canClose && (close === size || close >= 3)) {
      const inner = markdownInline(text.slice(open + size, k), context, depth + 1, inLink);
      const lead = text.slice(start, open);
      let node;
      if (ch === "~") node = el("del", {}, inner);
      else if (size === 3) node = el("strong", {}, el("em", {}, inner));
      else if (size === 2) node = el("strong", {}, inner);
      else node = el("em", {}, inner);
      return { node: lead ? [lead, node] : node, end: k + size };
    }
    k += close;
  }
  return asText;
}
