// The views' host (ADR-022): the view page, run inside the official MCP Apps
// app-bridge, the spec authors' own host side, rather than a stand-in of
// mm-mcp's, which would share mm-mcp's reading of the spec. The browser tests
// run the page in it, and the documentation shows the views live in it, with
// recorded data; it is bundled for each when they are built, and never
// shipped with mm-mcp.
//
// mount puts a page in a frame and plays a call's input and result to it.
// The host's answers to what the view asks, tools, pictures and links, come
// from whoever mounts it, and everything the view asks is recorded, for a
// test to read back.

import { AppBridge, PostMessageTransport } from "@modelcontextprotocol/ext-apps/app-bridge";

const frames = [];

const ALL_CAPABILITIES = {
  openLinks: {},
  serverTools: {},
  serverResources: {},
  updateModelContext: { text: {} },
  message: { text: {} },
};

// THEMES are a host's style variables, as a host sends them.
const THEMES = {
  light: { "--color-background-primary": "#ffffff", "--color-text-primary": "#141413", "--color-text-info": "#2f6fd1", "--font-sans": "system-ui, sans-serif" },
  dark: { "--color-background-primary": "#1f1f1e", "--color-text-primary": "#f1f1ef", "--color-text-info": "#85b8ff", "--font-sans": "system-ui, sans-serif" },
};

// ISOLATED_POLICY is what MCP Apps lets a view do when it declares no
// domains: run its own scripts and styles, show pictures it holds as data,
// and reach nothing.
const ISOLATED_POLICY = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; media-src data:; connect-src 'none'; frame-src 'none'; form-action 'none'";

// mount puts page in a frame, with these options, all of them optional:
//   container  the element the frame goes in, the page's body when not given
//   width      the frame's width in pixels; it fills its container otherwise
//   isolated   sandbox the frame as clients do, scripts alone, under
//              ISOLATED_POLICY, where a test cannot read what it drew
//   input      the arguments of the call the view belongs to
//   result     the call's result
//   tools      each tool's answers, in turn; the last one is kept
//   answer     a function (name, arguments) giving a tool's answer, or
//              nothing to fall back on tools
//   resources  each resource, by its address
//   without    capabilities the host leaves out
//   theme      "light" or "dark"; themes overrides the variables of each
//   locale     the host's language
//   refuseLinks  refuse to open links
function mount(page, options) {
  const settings = options || {};
  const iframe = document.createElement("iframe");
  // Same origin, so a test can read what the view drew; hosts give a view
  // scripts alone, as an isolated mount does.
  iframe.setAttribute("sandbox", settings.isolated ? "allow-scripts" : "allow-scripts allow-same-origin");
  iframe.setAttribute("title", settings.title || "A view of mm-mcp's");
  iframe.style.width = settings.width ? settings.width + "px" : settings.container ? "100%" : "640px";
  iframe.style.height = "100px";
  iframe.style.border = "0";
  iframe.style.display = "block";
  (settings.container || document.body).append(iframe);

  const themes = Object.assign({}, THEMES, settings.themes || {});
  const theme = settings.theme || "light";
  const frame = { iframe, calls: [], reads: [], links: [], contexts: [], messages: [], sizes: [], ready: false, themes };
  frames.push(frame);
  const index = frames.length - 1;

  const capabilities = Object.assign({}, ALL_CAPABILITIES);
  for (const name of settings.without || []) delete capabilities[name];
  frame.context = {
    theme,
    locale: settings.locale || "en-US",
    timeZone: "Europe/Amsterdam",
    displayMode: "inline",
    availableDisplayModes: ["inline"],
    platform: "desktop",
    styles: { variables: themes[theme] },
  };
  const bridge = new AppBridge(null, { name: "mm-mcp's host", version: "1" }, capabilities, { hostContext: frame.context });
  bridge.oncalltool = async (params) => {
    frame.calls.push(params);
    if (typeof settings.answer === "function") {
      const answer = settings.answer(params.name, params.arguments || {});
      if (answer) {
        if (answer.error) throw new Error(answer.error);
        return answer;
      }
    }
    const answers = (settings.tools || {})[params.name];
    if (!answers || answers.length === 0) throw new Error("the host answers no call of " + params.name);
    const answer = answers.length > 1 ? answers.shift() : answers[0];
    if (answer.error) throw new Error(answer.error);
    return answer;
  };
  bridge.onreadresource = async (params) => {
    frame.reads.push(params.uri);
    const found = (settings.resources || {})[params.uri];
    if (!found) throw new Error("Resource not found");
    return { contents: [Object.assign({ uri: params.uri }, found)] };
  };
  bridge.onopenlink = async (params) => {
    frame.links.push(params.url);
    return settings.refuseLinks ? { isError: true } : {};
  };
  bridge.onupdatemodelcontext = async (params) => {
    frame.contexts.push(params);
    return {};
  };
  bridge.onmessage = async (params) => {
    frame.messages.push(params);
    return {};
  };
  bridge.onsizechange = (params) => {
    frame.sizes.push(params);
    if (params && params.height > 0) iframe.style.height = Math.ceil(params.height) + "px";
  };
  bridge.oninitialized = async () => {
    await bridge.sendToolInput({ arguments: settings.input || {} });
    if (settings.result) await bridge.sendToolResult(settings.result);
    frame.ready = true;
  };
  frame.bridge = bridge;
  const connected = bridge.connect(new PostMessageTransport(iframe.contentWindow, iframe.contentWindow));
  iframe.srcdoc = settings.isolated ? isolate(page) : page;
  return connected.then(() => index);
}

// isolate puts the page under ISOLATED_POLICY, as a client's frame does.
function isolate(page) {
  const policy = '<meta http-equiv="Content-Security-Policy" content="' + ISOLATED_POLICY + '">';
  return page.replace(/<head>/i, "<head>" + policy);
}

// setTheme switches a frame's host between light and dark, as a host tells a
// view when its person switches.
function setTheme(index, theme) {
  const frame = frames[index];
  frame.context = Object.assign({}, frame.context, { theme, styles: { variables: frame.themes[theme] } });
  frame.bridge.setHostContext(frame.context);
}

// teardown asks the view to tear down, as a host does before it removes it.
async function teardown(index) {
  await frames[index].bridge.teardownResource({});
}

window.mmHost = { frames, mount, setTheme, teardown };
