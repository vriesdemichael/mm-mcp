// The view's side of MCP Apps (2026-01-26): JSON-RPC 2.0 over postMessage
// with the host that renders it. Written for this page, after
// bitbucket-data-center-cli's, rather than taken from the reference SDK, which
// would put its dependencies into every view; the protocol it covers is small
// and stable.

const APPS_PROTOCOL_VERSION = "2026-01-26";
const REQUEST_TIMEOUT_MS = 30000;

function createBridge(appInfo) {
  let nextID = 1;
  const pending = new Map();
  const listeners = new Map();

  function post(message) {
    // The host's origin is not known inside its sandbox, so "*" is the only
    // target there is. What leaves this way is the view's own requests.
    window.parent.postMessage(message, "*");
  }

  function request(method, params, timeout = REQUEST_TIMEOUT_MS) {
    const id = nextID++;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id);
        // Unanswered is not refused: the host may have passed the request on.
        const error = new Error(method + " got no answer");
        error.unanswered = true;
        reject(error);
      }, timeout);
      pending.set(id, {
        resolve: (value) => { clearTimeout(timer); resolve(value); },
        reject: (error) => { clearTimeout(timer); reject(error); },
      });
      post({ jsonrpc: "2.0", id, method, params });
    });
  }

  function notify(method, params) {
    post({ jsonrpc: "2.0", method, params });
  }

  function respond(id, result) {
    post({ jsonrpc: "2.0", id, result });
  }

  function emit(method, params) {
    for (const listener of listeners.get(method) || []) {
      try {
        listener(params);
      } catch (error) {
        console.error(error);
      }
    }
  }

  window.addEventListener("message", (event) => {
    // Only the host speaks to the view.
    if (event.source !== window.parent) return;
    const message = event.data;
    if (!message || typeof message !== "object" || message.jsonrpc !== "2.0") return;

    if (typeof message.method !== "string") {
      const waiting = pending.get(message.id);
      if (!waiting) return;
      pending.delete(message.id);
      if (message.error) {
        waiting.reject(new Error(message.error.message || "the host refused the request"));
      } else {
        waiting.resolve(message.result);
      }
      return;
    }

    if (message.id !== undefined && message.id !== null) {
      switch (message.method) {
        case "ping":
          respond(message.id, {});
          return;
        case "ui/resource-teardown":
          emit("teardown", message.params);
          respond(message.id, {});
          return;
        default:
          post({ jsonrpc: "2.0", id: message.id, error: { code: -32601, message: "Method not found" } });
          return;
      }
    }

    emit(message.method, message.params);
  });

  return {
    on(method, listener) {
      const list = listeners.get(method) || [];
      list.push(listener);
      listeners.set(method, list);
    },

    async connect(timeout) {
      const result = await request("ui/initialize", {
        appInfo,
        appCapabilities: { availableDisplayModes: ["inline", "fullscreen"] },
        protocolVersion: APPS_PROTOCOL_VERSION,
      }, timeout);
      notify("ui/notifications/initialized", {});
      return result || {};
    },

    reportSize(width, height) {
      notify("ui/notifications/size-changed", { width, height });
    },

    openLink(url) {
      return request("ui/open-link", { url });
    },

    // A call of the tool only views call, which the host passes to mm-mcp.
    callTool(name, args, timeout) {
      return request("tools/call", { name, arguments: args }, timeout);
    },

    // A picture of mm-mcp's, which the host reads from it: the view has no
    // network of its own.
    readResource(uri, timeout) {
      return request("resources/read", { uri }, timeout);
    },

    // The context the model reads on its next turn, in mm-mcp's words. Each
    // call replaces the last, and not every host passes it on.
    updateModelContext(text) {
      return request("ui/update-model-context", { content: [{ type: "text", text }] });
    },
  };
}
