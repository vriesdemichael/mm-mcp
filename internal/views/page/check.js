// The client check (ADR-022): what this client lets a view do, measured from
// inside a view, since the server sees none of what passes between a page and
// its host. check_views shows it when the person asks.
//
// What it does on its own only reads, its own fixtures and the person's own
// picture, and changes nothing. What opens something waits for a press. The
// report it makes goes out by every route it tests, and holds no credential,
// post or person, and not the server's address, because it gets pasted into
// issues.

const CONNECT_TIMEOUT_MS = 10000;
const RESULT_TIMEOUT_MS = 15000;
const CHECK_TIMEOUT_MS = 10000;
const SETTLE_MS = 1500;
const DETAIL_MOST = 300;

// CHECK_META is where a result carries what the check looks for in _meta, as
// the server names it.
const CHECK_META = "mm-mcp/check";
const REPORT_TOOL = "view_check";

// UNLISTED is an address the page declares nowhere, and which never resolves:
// nothing leaves the machine even in a host that enforces no policy.
const UNLISTED = "https://mm-mcp-check.invalid";
const PING_PATH = "/api/v4/system/ping";
const PICTURE_PATH = "/static/images/favicon/favicon-default-32x32.png";

const bridge = createBridge({ name: "mm-mcp check", version: "1" });

const state = {
  capabilities: {},
  context: {},
  hostInfo: null,
  protocol: "",
  // What came with the call this check belongs to, and how much of it.
  data: null,
  arrived: null,
  rows: new Map(),
  changes: [],
  violations: [],
  standalone: false,
  measuring: false,
  finished: false,
  avatar: "",
  copied: "",
};

const AUTOMATIC = [
  { id: "host", label: () => t("check.row_host") },
  { id: "frame", label: () => t("check.row_frame") },
  { id: "page_age", label: () => t("check.row_page_age") },
  { id: "result", label: () => t("check.row_result") },
  { id: "read_page", label: () => t("check.row_read_page") },
  { id: "read_image", label: () => t("check.row_read_image") },
  { id: "read_text", label: () => t("check.row_read_text") },
  { id: "read_pdf", label: () => t("check.row_read_pdf") },
  { id: "read_template", label: () => t("check.row_read_template") },
  { id: "tool_call", label: () => t("check.row_tool_call") },
  { id: "data_image", label: () => t("check.row_data_image") },
  { id: "fetch_server", label: () => t("check.row_fetch_server") },
  { id: "image_server", label: () => t("check.row_image_server") },
  { id: "fetch_unlisted", label: () => t("check.row_fetch_unlisted") },
  { id: "image_unlisted", label: () => t("check.row_image_unlisted") },
  { id: "policy", label: () => t("check.row_policy") },
  { id: "model_context", label: () => t("check.row_model_context") },
  { id: "size", label: () => t("check.row_size") },
];

// Declared and never tried: mm-mcp never sends a message in the person's name,
// and a log message has no answer to measure.
const DECLARED_ONLY = [
  { id: "message", capability: "message", label: () => t("check.row_message") },
  { id: "logging", capability: "logging", label: () => t("check.row_logging") },
];

function otherScheme() {
  return SERVER.startsWith("https:") ? SERVER.replace(/^https:/, "http:") : SERVER.replace(/^http:/, "https:");
}

const PRESSES = [
  { id: "open_link", label: () => t("check.row_open_link"), press: () => pressLink("open_link", SERVER) },
  { id: "open_link_other", label: () => t("check.row_open_link_other"), press: () => pressLink("open_link_other", otherScheme()) },
  { id: "plain_link", label: () => t("check.row_plain_link"), anchor: true },
  { id: "window_open", label: () => t("check.row_window_open"), press: pressWindowOpen },
  { id: "clipboard", label: () => t("check.row_clipboard"), press: pressClipboard },
  { id: "display_fullscreen", label: () => t("check.row_display_fullscreen"), press: () => pressDisplay("display_fullscreen", "fullscreen") },
  { id: "display_inline", label: () => t("check.row_display_inline"), press: () => pressDisplay("display_inline", "inline"), only: () => state.context.displayMode === "fullscreen" },
];

// Status words, by what a row found.
function statusWord(status) {
  switch (status) {
    case "works": return t("check.status_works");
    case "fails": return t("check.status_fails");
    case "partly": return t("check.status_partly");
    case "reached": return t("check.status_reached");
    case "blocked": return t("check.status_blocked");
    case "no_answer": return t("check.status_no_answer");
    case "declared": return t("check.status_declared");
    case "not_declared": return t("check.status_not_declared");
    case "skipped": return t("check.status_skipped");
    case "asking": return t("check.status_asking");
    case "not_tried": return t("check.status_not_tried");
    default: return t("check.status_running");
  }
}

// scrub takes the server's address and the person's picture out of what is
// reported: a host's error may repeat what it was asked for.
function scrub(text) {
  const replace = (value, name) => { if (value) text = text.split(value).join(name); };
  replace(state.avatar, "<picture>");
  replace(SERVER, "<server>");
  replace(otherScheme(), "<server>");
  replace(RESOURCE_BASE, "<server>");
  try {
    replace(new URL(SERVER).host, "<server>");
  } catch {
    // An address that does not parse is reported as it stands, scrubbed above.
  }
  return text;
}

function messageOf(error) {
  const text = error && error.message ? String(error.message) : String(error);
  return error && error.unanswered ? t("check.no_answer") : text;
}

function setRow(id, status, detail) {
  state.rows.set(id, { status, detail: scrub(String(detail || "")).slice(0, DETAIL_MOST) });
  render();
}

function statusOf(id) {
  const row = state.rows.get(id);
  return row ? row.status : "";
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

document.addEventListener("securitypolicyviolation", (event) => {
  state.violations.push({ directive: event.effectiveDirective || event.violatedDirective, blocked: event.blockedURI });
});

let resultCame;
const resultArrives = new Promise((resolve) => { resultCame = resolve; });

bridge.on("ui/notifications/tool-result", (result) => {
  const given = result && typeof result === "object" ? result : {};
  const meta = given._meta && typeof given._meta === "object" ? given._meta[CHECK_META] : null;
  state.arrived = {
    content: Array.isArray(given.content) && given.content.length > 0,
    structuredContent: Boolean(given.structuredContent && typeof given.structuredContent === "object"),
    meta: Boolean(meta && meta.stamp),
  };
  state.data = state.arrived.structuredContent ? given.structuredContent : null;
  state.metaStamp = meta && typeof meta.stamp === "string" ? meta.stamp : "";
  state.avatar = state.data && typeof state.data.avatar_uri === "string" ? state.data.avatar_uri : "";
  resultCame();
});

bridge.on("ui/notifications/host-context-changed", (context) => {
  applyHostContext(context);
  if (context && (context.displayMode || context.containerDimensions)) {
    state.changes.push({ display_mode: context.displayMode || state.context.displayMode || "", container_dimensions: context.containerDimensions || null });
    render();
    share();
  }
});

function applyHostContext(context) {
  if (!context || typeof context !== "object") return;
  state.context = Object.assign({}, state.context, context);
  const root = document.documentElement;
  if (context.theme === "light" || context.theme === "dark") root.dataset.theme = context.theme;
  const styles = context.styles || {};
  for (const [name, value] of Object.entries(styles.variables || {})) {
    if (name.startsWith("--") && typeof value === "string") root.style.setProperty(name, value);
  }
  if (styles.css && typeof styles.css.fonts === "string") {
    document.getElementById("host-fonts").textContent = styles.css.fonts;
  }
  if (typeof context.locale === "string") setLanguage(context.locale);
  if (typeof context.displayMode === "string") root.dataset.display = context.displayMode;
  render();
}

// What the checks that run on their own find.

function checkHost() {
  const info = state.hostInfo;
  const name = info ? [info.name, info.version].filter(Boolean).join(" ") : t("check.unnamed");
  setRow("host", "works", t("check.host_detail", { name, protocol: state.protocol || "?", modes: (state.context.availableDisplayModes || []).join(", ") || "?" }));
  for (const row of DECLARED_ONLY) {
    setRow(row.id, state.capabilities[row.capability] ? "declared" : "not_declared", "");
  }
}

function checkFrame() {
  const ancestors = location.ancestorOrigins ? [...location.ancestorOrigins].join(", ") : "";
  setRow("frame", "works", t("check.frame_detail", { origin: location.origin, ancestors: ancestors || "?" }));
}

async function checkResult() {
  await Promise.race([resultArrives, wait(RESULT_TIMEOUT_MS)]);
  const arrived = state.arrived;
  if (!arrived) {
    setRow("result", "fails", t("check.result_none"));
    setRow("page_age", "skipped", "");
    return;
  }
  const parts = [];
  if (arrived.content) parts.push("content");
  if (arrived.structuredContent) parts.push("structuredContent");
  if (arrived.meta) parts.push("_meta");
  const missing = ["content", "structuredContent", "_meta"].filter((part) => !parts.includes(part));
  setRow("result", missing.length === 0 ? "works" : "partly", missing.length === 0
    ? t("check.result_all")
    : t("check.result_missing", { missing: missing.join(", ") }));
  const given = state.data && typeof state.data.stamp === "string" ? state.data.stamp : state.metaStamp;
  if (!given) setRow("page_age", "skipped", "");
  else if (given === STAMP) setRow("page_age", "works", t("check.page_current"));
  else setRow("page_age", "fails", t("check.page_older"));
}

function decodedSize(content) {
  try {
    return atob(content.blob).length;
  } catch {
    return -1;
  }
}

// checkRead reads a resource through the host, and holds what comes to what
// matches says it should be.
async function checkRead(id, uri, matches) {
  if (!uri) {
    setRow(id, "skipped", t("check.read_no_address"));
    return;
  }
  try {
    const answer = await bridge.readResource(uri, CHECK_TIMEOUT_MS);
    const content = answer && Array.isArray(answer.contents) ? answer.contents[0] : null;
    if (!content) {
      setRow(id, "fails", t("check.read_empty"));
    } else if (matches(content)) {
      setRow(id, "works", content.mimeType || "");
    } else {
      setRow(id, "fails", t("check.read_wrong", { type: content.mimeType || "?" }));
    }
  } catch (error) {
    setRow(id, "fails", messageOf(error));
  }
}

function fixtureMatches(fixture) {
  return (content) => {
    if (content.mimeType !== fixture.type) return false;
    return fixture.text ? content.text === fixture.text : decodedSize(content) === fixture.size;
  };
}

async function checkReads() {
  await checkRead("read_page", PAGE_URI, (content) => typeof content.text === "string" && content.text.startsWith("<!doctype html>"));
  await checkRead("read_image", FIXTURES.image.uri, fixtureMatches(FIXTURES.image));
  await checkRead("read_text", FIXTURES.text.uri, fixtureMatches(FIXTURES.text));
  await checkRead("read_pdf", FIXTURES.pdf.uri, fixtureMatches(FIXTURES.pdf));
  await checkRead("read_template", state.avatar, (content) => String(content.mimeType).startsWith("image/") && decodedSize(content) > 0);
}

async function checkToolCall() {
  try {
    const answer = await bridge.callTool(REPORT_TOOL, {}, CHECK_TIMEOUT_MS);
    if (!answer || answer.isError) {
      setRow("tool_call", "fails", textOf(answer) || t("check.tool_refused"));
      return;
    }
    const counted = answer.structuredContent || {};
    setRow("tool_call", "works", t("check.tool_reached", { reads: String(counted.fixture_reads ?? "?"), calls: String(counted.calls ?? "?") }));
  } catch (error) {
    setRow("tool_call", "fails", messageOf(error));
  }
}

function textOf(result) {
  const parts = result && Array.isArray(result.content) ? result.content : [];
  return parts.filter((part) => part && part.type === "text").map((part) => part.text).join("\n");
}

function loadImage(url) {
  return new Promise((resolve) => {
    const image = new Image();
    const timer = setTimeout(() => resolve(false), CHECK_TIMEOUT_MS);
    image.onload = () => { clearTimeout(timer); resolve(true); };
    image.onerror = () => { clearTimeout(timer); resolve(false); };
    image.src = url;
  });
}

async function checkDataImage() {
  setRow("data_image", (await loadImage(DOT)) ? "works" : "fails", "");
}

// reachOutcome says what stopped an attempt on an address, once any policy
// violation has had time to arrive.
async function reachOutcome(id, url) {
  await wait(50);
  const origin = new URL(url).origin;
  // By the address asked for, since a fetch and a picture from one server are
  // stopped by different directives. A browser may give the origin alone.
  const violation = state.violations.findLast((each) => each.blocked === url || each.blocked === origin);
  if (violation) setRow(id, "blocked", t("check.blocked_by", { directive: violation.directive }));
  else setRow(id, "no_answer", t("check.unreachable"));
}

async function checkFetch(id, url) {
  try {
    // no-cors: this asks whether the frame may reach the address, whatever the
    // address says of cross-origin reads, and sends no cookie.
    await fetch(url, { mode: "no-cors", credentials: "omit", cache: "no-store", signal: AbortSignal.timeout(CHECK_TIMEOUT_MS) });
    setRow(id, "reached", "");
  } catch {
    await reachOutcome(id, url);
  }
}

async function checkImage(id, url) {
  if (await loadImage(url)) setRow(id, "reached", "");
  else await reachOutcome(id, url);
}

async function checkNetwork() {
  await checkFetch("fetch_server", SERVER + PING_PATH);
  await checkImage("image_server", SERVER + PICTURE_PATH);
  await checkFetch("fetch_unlisted", UNLISTED + "/ping");
  await checkImage("image_unlisted", UNLISTED + "/picture.png");
}

// checkPolicy holds what the host says it approved to what got through.
function checkPolicy() {
  const sandbox = state.capabilities.sandbox;
  const csp = sandbox && typeof sandbox === "object" ? sandbox.csp : null;
  if (!csp || typeof csp !== "object") {
    setRow("policy", "not_declared", "");
    return;
  }
  let origin = "";
  try {
    origin = new URL(SERVER).origin;
  } catch {
    origin = SERVER;
  }
  const lists = (domains) => Array.isArray(domains) && domains.some((domain) => String(domain).startsWith(origin));
  const stopped = (id) => statusOf(id) === "blocked";
  const betrayed = (lists(csp.connectDomains) && stopped("fetch_server")) || (lists(csp.resourceDomains) && stopped("image_server"));
  if (betrayed) setRow("policy", "partly", t("check.policy_betrayed"));
  else setRow("policy", "works", t("check.policy_holds"));
}

// checkSize has the view grow and shrink, as a long post or an opened image
// does, and sees whether the frame follows and whether a scrollbar stays. A host
// may hold a view to a height of its own, past which the view scrolls inside.
async function checkSize() {
  const root = document.documentElement;
  reportHeight();
  await wait(SETTLE_MS);
  // A host may run a view again out of sight, as when a conversation is
  // redrawn, and a frame out of sight says nothing of how one in sight behaves.
  if (document.visibilityState === "hidden") {
    setRow("size", "skipped", t("check.size_hidden"));
    return;
  }
  const before = window.innerHeight;
  if (before + 1 < document.getElementById("app").getBoundingClientRect().height) {
    setRow("size", "partly", t("check.size_capped", { height: String(before) }));
    return;
  }
  root.dataset.measuring = "true";
  state.measuring = true;
  render();
  reportHeight();
  await wait(SETTLE_MS);
  const grown = window.innerHeight;
  state.measuring = false;
  render();
  reportHeight();
  await wait(SETTLE_MS);
  const after = window.innerHeight;
  const scrollbar = window.innerWidth - root.clientWidth;
  delete root.dataset.measuring;
  const followed = grown > before;
  const detail = t("check.size_detail", { before: String(before), grown: String(grown), after: String(after), scrollbar: String(scrollbar) });
  if (!followed) setRow("size", "fails", detail);
  else if (scrollbar > 0) setRow("size", "partly", detail);
  else setRow("size", "works", detail);
  render();
}

// What the presses find.

async function pressLink(id, url) {
  setRow(id, "");
  try {
    const answer = await bridge.openLink(url);
    const scheme = url.split(":")[0];
    if (answer && answer.isError) setRow(id, "fails", t("check.link_refused", { scheme }));
    else setRow(id, "works", t("check.link_answered", { scheme }));
  } catch (error) {
    setRow(id, "fails", messageOf(error));
  }
  share();
}

// Whether a plain link or a window opened, only the person can say.
function askWhether(id, detail) {
  setRow(id, "asking", detail);
}

function pressWindowOpen() {
  let opened = null;
  try {
    opened = window.open(SERVER, "_blank", "noopener");
  } catch {
    opened = null;
  }
  askWhether("window_open", opened ? "" : t("check.window_null"));
}

async function pressClipboard() {
  setRow("clipboard", "");
  try {
    await navigator.clipboard.writeText("mm-mcp client check");
    setRow("clipboard", "works", "");
  } catch (error) {
    setRow("clipboard", "fails", messageOf(error));
  }
  share();
}

async function pressDisplay(id, mode) {
  const from = state.context.displayMode || "?";
  setRow(id, "");
  try {
    const answer = await bridge.requestDisplayMode(mode, CHECK_TIMEOUT_MS);
    const given = answer && typeof answer.mode === "string" ? answer.mode : "?";
    await wait(SETTLE_MS);
    const now = state.context.displayMode || "?";
    const detail = t("check.display_detail", { from, mode, given, now });
    setRow(id, given === mode && now === mode ? "works" : "fails", detail);
  } catch (error) {
    setRow(id, "fails", messageOf(error));
  }
  share();
}

// What the report says, and where it goes.

function effects() {
  const out = [];
  if (statusOf("page_age") === "fails") out.push(t("check.effect_page"));
  if (state.arrived && !state.arrived.structuredContent) out.push(t("check.effect_data"));
  if (statusOf("tool_call") === "fails") out.push(t("check.effect_tools"));
  if (statusOf("read_image") === "fails") {
    out.push(state.data && state.data.carries_pictures ? t("check.effect_pictures_carried") : t("check.effect_pictures"));
  }
  if (!state.capabilities.openLinks || statusOf("open_link") === "fails") out.push(t("check.effect_links"));
  if (statusOf("model_context") === "fails") out.push(t("check.effect_model"));
  const modes = state.context.availableDisplayModes;
  if (Array.isArray(modes) && !modes.includes("fullscreen")) out.push(t("check.effect_fullscreen"));
  return out;
}

function reportText() {
  const checks = {};
  for (const [id, row] of state.rows) checks[id] = row.detail ? { status: row.status, detail: row.detail } : { status: row.status };
  const report = {
    check: "mm-mcp client check",
    mm_mcp: RELEASE,
    server_scheme: SERVER.startsWith("https:") ? "https" : "http",
    host_capabilities: state.capabilities,
    host_context_keys: Object.keys(state.context),
    available_display_modes: state.context.availableDisplayModes || null,
    container_dimensions: state.context.containerDimensions || null,
    platform: state.context.platform || null,
    checks,
    display_changes: state.changes,
    effects: effects(),
    not_measured: ["whether the model sees _meta or structuredContent", "where the client stores a result on disk"],
  };
  return scrub(JSON.stringify(report, null, 1));
}

let sharing = false;
let shareAgain = false;

// share sends the report out by every route there is, again after each press.
// Each route's failure is its own: none of them stands in the way of another.
async function share() {
  if (sharing) {
    shareAgain = true;
    return;
  }
  sharing = true;
  const report = reportText();
  try {
    const answer = await bridge.updateModelContext(t("check.model_context", { report }));
    if (answer && answer.isError) throw new Error(t("check.tool_refused"));
    // The report that says this has yet to go.
    if (statusOf("model_context") !== "works") {
      setRow("model_context", "works", t("check.context_answered"));
      shareAgain = true;
    }
  } catch (error) {
    if (statusOf("model_context") !== "fails") setRow("model_context", "fails", messageOf(error));
  }
  if (statusOf("tool_call") === "works") {
    try {
      await bridge.callTool(REPORT_TOOL, { report: reportText() }, CHECK_TIMEOUT_MS);
    } catch {
      // The report is on the page, for the person to paste.
    }
  }
  sharing = false;
  if (shareAgain) {
    shareAgain = false;
    share();
  }
}

// selectReport copies the report where the host lets it, and leaves it selected
// to copy by hand where it does not.
async function selectReport(area) {
  try {
    await navigator.clipboard.writeText(area.value);
    state.copied = t("check.copied");
  } catch {
    state.copied = t("check.selected");
  }
  render();
  const again = document.querySelector(".check-report");
  again.focus();
  again.select();
}

// The page.

function rowOf(item, control) {
  const row = state.rows.get(item.id);
  const status = row ? row.status : "";
  return el("div", { class: "check-row", dataset: { check: item.id } },
    el("span", { class: "check-label" }, item.label()),
    el("span", { class: "check-status", dataset: { status: status || "running" } }, control && !row ? statusWord("not_tried") : statusWord(status)),
    el("span", { class: "check-detail" }, control || null, status === "asking" ? whetherButtons(item.id) : null, row ? row.detail : ""));
}

function whetherButtons(id) {
  const answer = (opened) => () => {
    setRow(id, opened ? "works" : "fails", opened ? t("check.person_opened") : t("check.person_nothing"));
    share();
  };
  return [
    t("check.did_it_open"), " ",
    el("button", { type: "button", class: "check-press", onclick: answer(true) }, t("check.yes")),
    el("button", { type: "button", class: "check-press", onclick: answer(false) }, t("check.no")),
  ];
}

// plainLink is the one anchor anywhere in mm-mcp's views, here to see what a
// host's frame does with one.
function plainLink() {
  const anchor = document.createElement("a");
  anchor.href = SERVER;
  anchor.target = "_blank";
  anchor.rel = "noopener noreferrer";
  anchor.className = "check-press";
  anchor.textContent = t("check.press");
  anchor.addEventListener("click", () => askWhether("plain_link", ""));
  return anchor;
}

function controlOf(item) {
  if (item.anchor) return plainLink();
  const usable = !item.only || item.only();
  return el("button", { type: "button", class: "check-press", disabled: !usable, onclick: () => item.press(), title: usable ? null : t("check.fullscreen_first") }, t("check.press"));
}

function content() {
  if (state.standalone) return [el("p", { class: "notice", role: "status" }, t("check.standalone"))];
  const said = effects();
  const area = el("textarea", { class: "check-report", readonly: true, "aria-label": t("check.report") });
  area.value = reportText();
  return [
    el("h2", { class: "check-title" }, t("check.title")),
    el("p", { class: "check-lead muted", role: "status" }, state.finished ? t("check.finished") : t("check.lead")),
    said.length > 0 ? el("ul", { class: "check-effects", "aria-label": t("check.effects") }, said.map((line) => el("li", {}, line))) : null,
    el("h3", { class: "check-group" }, t("check.group_automatic")),
    AUTOMATIC.map((item) => rowOf(item)),
    el("h3", { class: "check-group" }, t("check.group_presses")),
    PRESSES.map((item) => rowOf(item, controlOf(item))),
    el("h3", { class: "check-group" }, t("check.group_declared")),
    DECLARED_ONLY.map((item) => rowOf(item)),
    el("h3", { class: "check-group" }, t("check.report")),
    area,
    el("button", { type: "button", class: "check-press", onclick: () => selectReport(area) }, t("check.copy")),
    state.copied ? el("span", { class: "muted", role: "status" }, " ", state.copied) : null,
    state.measuring ? el("div", { class: "check-spacer" }) : null,
  ].flat().filter(Boolean);
}

function render() {
  const app = document.getElementById("app");
  // A press that redraws the page keeps the person's place on the row.
  const focused = document.activeElement && document.activeElement.closest ? document.activeElement.closest("[data-check]") : null;
  const place = focused ? focused.dataset.check : "";
  app.replaceChildren(...content());
  if (place) {
    const again = app.querySelector("[data-check=\"" + place + "\"] .check-press");
    if (again) again.focus({ preventScroll: true });
  }
}

// The host sizes an inline view to what it reports.
function reportHeight() {
  if (document.documentElement.dataset.display === "fullscreen") return;
  const box = document.getElementById("app").getBoundingClientRect();
  bridge.reportSize(Math.ceil(document.documentElement.clientWidth), Math.ceil(box.height));
}

function watchSize() {
  let queued = false;
  new ResizeObserver(() => {
    if (!queued) {
      queued = true;
      requestAnimationFrame(() => {
        queued = false;
        reportHeight();
      });
    }
  }).observe(document.getElementById("app"));
}

async function start() {
  setLanguage(navigator.language);
  render();
  watchSize();
  try {
    const answer = await bridge.connect(CONNECT_TIMEOUT_MS);
    state.capabilities = answer.hostCapabilities && typeof answer.hostCapabilities === "object" ? answer.hostCapabilities : {};
    state.hostInfo = answer.hostInfo || null;
    state.protocol = typeof answer.protocolVersion === "string" ? answer.protocolVersion : "";
    applyHostContext(answer.hostContext);
  } catch {
    state.standalone = true;
    render();
    return;
  }
  checkHost();
  checkFrame();
  await checkResult();
  await checkReads();
  await checkToolCall();
  await checkDataImage();
  await checkNetwork();
  checkPolicy();
  await checkSize();
  await share();
  state.finished = true;
  render();
}

start();
