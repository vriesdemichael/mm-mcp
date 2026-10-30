// The view: it waits for the result of the call it belongs to, and draws it.
//
// What a person does here goes through the host: a link opens through
// ui/open-link, a picture is read through resources/read, and more of a
// thread, or a channel or a thread opened in place, comes from the tools only
// views call. The view never speaks for the
// person, so it never writes in the conversation in their name, and it acts
// only on a click: hosts run a view's code again when a conversation is
// reopened.

const CONNECT_TIMEOUT_MS = 10000;
const NO_RESULT_MS = 60000;

const bridge = createBridge({ name: "mm-mcp", version: "1" });

const view = {
  // What the host lets the view do, from its answer to ui/initialize.
  readsResources: false,
  callsTools: false,
  opensLinks: true,
  tellsModel: false,
  // What the view draws: the call's arguments, and its result.
  input: null,
  result: null,
  // The results a quote was opened from, latest last, for the way back.
  trail: [],
  error: "",
  waited: false,
  standalone: false,
  notice: null,
  lightbox: null,
};

bridge.on("ui/notifications/tool-input", (params) => {
  view.input = params && typeof params.arguments === "object" ? params.arguments : {};
  render();
});

bridge.on("ui/notifications/tool-result", (result) => {
  view.result = null;
  view.trail = [];
  view.error = "";
  if (!result || typeof result !== "object") {
    view.error = t("view.nothing");
  } else if (result.isError) {
    view.error = textOf(result) || t("view.failed");
  } else if (result.structuredContent && typeof result.structuredContent === "object") {
    view.result = result.structuredContent;
  } else {
    view.error = textOf(result) || t("view.nothing");
  }
  render();
});

bridge.on("ui/notifications/tool-cancelled", () => {
  view.error = t("view.cancelled");
  render();
});

bridge.on("ui/notifications/host-context-changed", (context) => applyHostContext(context));

function textOf(result) {
  const parts = Array.isArray(result.content) ? result.content : [];
  return parts.filter((part) => part && part.type === "text" && typeof part.text === "string").map((part) => part.text).join("\n");
}

function applyHostContext(context) {
  if (!context || typeof context !== "object") return;
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
  render();
}

// notice is a line the view shows of what it could not do here, with where
// to do it instead.
function notice(text, url) {
  view.notice = { text, url: url || "" };
  render();
}

function render() {
  const app = document.getElementById("app");
  app.replaceChildren(...content());
}

function content() {
  const out = [];
  if (view.notice) {
    out.push(el("p", { class: "notice", role: "status" },
      view.notice.text,
      view.notice.url ? [" ", linkButton(view.notice.url, [t("notice.open")])] : null,
      el("button", { type: "button", class: "link dismiss", "aria-label": t("notice.dismiss"), onclick: () => { view.notice = null; render(); } }, "×")));
  }
  if (view.lightbox) out.push(view.lightbox);
  if (view.error) {
    out.push(el("p", { class: "error", role: "alert" }, view.error));
  } else if (view.result) {
    out.push(renderKind(view.result));
  } else if (view.waited) {
    out.push(el("p", { class: "muted", role: "status" }, t("view.no_result")));
  } else {
    out.push(el("div", { class: "skeleton", "aria-busy": "true", "aria-label": t("view.loading") },
      el("span", { class: "skeleton-line" }), el("span", { class: "skeleton-line short" })));
  }
  return out;
}

// renderKind draws the kind of view the call asked for: show names it, a
// tool that posts answers with the post it posted, and propose_post with the
// draft it proposed.
function renderKind(data) {
  const named = view.input && typeof view.input.kind === "string" ? view.input.kind : "";
  const kind = named || (typeof data.saved === "boolean" && typeof data.channel_id === "string" ? "proposal"
    : typeof data.id === "string" && typeof data.author_id === "string" ? "posted" : "thread");
  switch (kind) {
    case "proposal":
      return renderProposal(data);
    case "posted":
      return renderPosted(data);
    case "thread":
      return renderThread(data);
    case "unreads":
      return renderUnreads(data);
    default:
      return el("p", { class: "muted", role: "status" }, t("view.unknown_kind", { kind }));
  }
}

// The host sizes an inline view to what it reports.
function watchSize() {
  let queued = false;
  const report = () => {
    queued = false;
    const box = document.getElementById("app").getBoundingClientRect();
    bridge.reportSize(Math.ceil(document.documentElement.clientWidth), Math.ceil(box.height));
  };
  new ResizeObserver(() => {
    if (!queued) {
      queued = true;
      requestAnimationFrame(report);
    }
  }).observe(document.getElementById("app"));
}

async function start() {
  setLanguage(navigator.language);
  render();
  watchSize();
  setTimeout(() => {
    if (!view.result && !view.error) {
      view.waited = true;
      render();
    }
  }, NO_RESULT_MS);
  try {
    const result = await bridge.connect(CONNECT_TIMEOUT_MS);
    // A host that says what it can do, and leaves something out, does not do
    // it: its links are shown to open by hand, its pictures are initials, and
    // more of a thread opens in Mattermost.
    const capabilities = result.hostCapabilities;
    if (capabilities && typeof capabilities === "object") {
      view.opensLinks = Boolean(capabilities.openLinks);
      view.callsTools = Boolean(capabilities.serverTools);
      view.readsResources = Boolean(capabilities.serverResources);
      view.tellsModel = Boolean(capabilities.updateModelContext);
    }
    applyHostContext(result.hostContext);
  } catch {
    view.standalone = true;
    render();
  }
}

start();
