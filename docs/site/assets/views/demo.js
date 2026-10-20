// mm-mcp's views, live in the documentation (mm-mcp#38). Each element with
// a data-view attribute shows one: the real view page mm-mcp serves, run by
// the host its browser tests run it in, sandboxed as an AI app sandboxes it,
// and played a recording of what the view drew on a real Mattermost. Nothing
// leaves the page: the host answers the view's presses from the recording,
// and an edit plays out into the next step.
(() => {
  "use strict";

  const here = document.currentScript ? new URL(".", document.currentScript.src) : null;
  const holders = [...document.querySelectorAll(".mm-view[data-view]")];
  if (!here || holders.length === 0) return;

  // The docs' own colours and type, as an AI app hands its theme to a view.
  function themeOf() {
    const style = getComputedStyle(document.body);
    const value = (name, fallback) => style.getPropertyValue(name).trim() || fallback;
    return {
      "--color-background-primary": value("--md-default-bg-color", "#ffffff"),
      "--color-background-secondary": value("--md-code-bg-color", "#f4f5f6"),
      "--color-text-primary": value("--md-default-fg-color", "#141413"),
      "--color-text-secondary": value("--md-default-fg-color--light", "#6b6e78"),
      "--color-text-info": value("--md-typeset-a-color", "#2f6fd1"),
      "--color-border-tertiary": value("--md-default-fg-color--lightest", "#e3e4e8"),
      "--font-sans": "'IBM Plex Sans', system-ui, sans-serif",
      "--font-mono": "'IBM Plex Mono', ui-monospace, monospace",
    };
  }

  function scheme() {
    return document.body.getAttribute("data-md-color-scheme") === "slate" ? "dark" : "light";
  }

  function load(src) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.onload = resolve;
      script.onerror = () => reject(new Error("could not load " + src));
      document.head.append(script);
    });
  }

  // played is a tool's result as a host hands it to the view.
  function played(structured) {
    return { content: [], structuredContent: structured };
  }

  // player answers what the view asks from its recording: an edit is saved
  // as the visitor wrote it, a discard is done, Post confirms first where the
  // recording did and then posts what the view shows, and anything else is
  // answered as recorded, the last answer kept.
  function player(recording) {
    const answers = recording.answers || {};
    const turns = {};
    const proposed = recording.result && recording.result.structuredContent;
    return (name, args) => {
      const key = name + ":" + (args.action || "");
      if (key === "view_change:edit") {
        const draft = Object.assign({}, proposed, { message: args.message, length: Array.from(args.message || "").length });
        delete draft.saved;
        delete draft.theirs;
        return played({ status: "saved", draft });
      }
      if (key === "view_change:discard") return played({ status: "discarded" });
      const list = answers[key];
      if (!list || list.length === 0) return { error: "the documentation plays no " + key };
      let answer;
      if (key === "view_change:post") {
        answer = args.confirmed ? list.find((a) => a.structuredContent && a.structuredContent.status === "posted") : list[0];
        answer = JSON.parse(JSON.stringify(answer || list[list.length - 1]));
        const post = answer.structuredContent && answer.structuredContent.post;
        if (post && typeof args.shown === "string") post.message = args.shown;
        return answer;
      }
      const turn = Math.min(turns[key] || 0, list.length - 1);
      turns[key] = turn + 1;
      return JSON.parse(JSON.stringify(list[turn]));
    };
  }

  async function mountIn(holder, page) {
    const name = holder.getAttribute("data-view");
    const response = await fetch(new URL("recordings/" + name + ".json", here));
    if (!response.ok) throw new Error("no recording " + name);
    const recording = await response.json();
    holder.replaceChildren();
    const frame = document.createElement("div");
    frame.className = "mm-view__frame";
    holder.append(frame);
    const theme = scheme();
    const index = await window.mmHost.mount(page, {
      container: frame,
      isolated: true,
      title: holder.getAttribute("data-title") || "A view of mm-mcp's",
      input: recording.input,
      result: recording.result,
      resources: recording.resources,
      answer: player(recording),
      without: holder.hasAttribute("data-no-presses") ? ["serverTools"] : [],
      theme,
      themes: { [theme]: themeOf() },
      locale: "en-US",
    });
    holder.dataset.frame = String(index);
    const again = document.createElement("button");
    again.type = "button";
    again.className = "mm-view__again";
    again.textContent = "Start over";
    again.addEventListener("click", () => mountIn(holder, page));
    holder.append(again);
  }

  async function start() {
    try {
      await load(new URL("host.js", here).href);
      const response = await fetch(new URL("page.html", here));
      if (!response.ok) throw new Error("no view page");
      const page = await response.text();
      await Promise.all(holders.map((holder) => mountIn(holder, page)));
    } catch (error) {
      for (const holder of holders) {
        holder.textContent = "The live view could not be loaded here: " + error.message;
      }
      return;
    }
    // The docs' light and dark switch reaches each view as its host's theme.
    new MutationObserver(() => {
      const theme = scheme();
      for (const holder of holders) {
        const index = Number(holder.dataset.frame);
        const frame = window.mmHost.frames[index];
        if (!frame) continue;
        frame.themes[theme] = themeOf();
        window.mmHost.setTheme(index, theme);
      }
    }).observe(document.body, { attributes: true, attributeFilter: ["data-md-color-scheme"] });
  }

  start();
})();
