// The views browser tests' host (ADR-022): the view page, run inside the
// official MCP Apps app-bridge, the spec authors' own host side, rather than
// a stand-in of mm-mcp's, which would share mm-mcp's reading of the spec. It
// is bundled for the browser when the tests run and never shipped.
//
// mount puts a page in a frame and plays a call's input and result to it.
// The host's answers to what the view asks, tools, pictures and links, come
// from the test, and everything the view asks is recorded for the test to
// read back.

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

async function mount(page, options) {
  const settings = options || {};
  const iframe = document.createElement("iframe");
  // Same origin, so the test can read what the view drew; hosts give a view
  // scripts alone.
  iframe.setAttribute("sandbox", "allow-scripts allow-same-origin");
  iframe.style.width = (settings.width || 640) + "px";
  iframe.style.height = "100px";
  iframe.style.border = "0";
  iframe.style.display = "block";
  document.body.append(iframe);

  const frame = { iframe, calls: [], reads: [], links: [], contexts: [], messages: [], sizes: [], ready: false };
  frames.push(frame);

  const capabilities = Object.assign({}, ALL_CAPABILITIES);
  for (const name of settings.without || []) delete capabilities[name];
  const theme = settings.theme || "light";
  const bridge = new AppBridge(null, { name: "mm-mcp test host", version: "1" }, capabilities, {
    hostContext: {
      theme,
      locale: settings.locale || "en-US",
      timeZone: "Europe/Amsterdam",
      displayMode: "inline",
      availableDisplayModes: ["inline"],
      platform: "desktop",
      styles: { variables: THEMES[theme] },
    },
  });
  bridge.oncalltool = async (params) => {
    frame.calls.push(params);
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
  await bridge.connect(new PostMessageTransport(iframe.contentWindow, iframe.contentWindow));
  frame.bridge = bridge;
  iframe.srcdoc = page;
  return frames.length - 1;
}

// teardown asks the view to tear down, as a host does before it removes it.
async function teardown(index) {
  await frames[index].bridge.teardownResource({});
}

window.mmHost = { frames, mount, teardown };
