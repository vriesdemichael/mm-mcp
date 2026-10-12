// Spike harness (#37): a chat model for VS Code backed by the scripted model
// endpoint, so VS Code's chat calls tools with no inference involved.
const vscode = require("vscode");
const ENDPOINT = "http://127.0.0.1:18080/v1/chat/completions";
function text(message) {
  return message.content.map((part) => {
    if (part instanceof vscode.LanguageModelTextPart) return part.value;
    if (part instanceof vscode.LanguageModelToolResultPart) return part.content.map((c) => "[" + c.constructor.name + (c.mimeType ? " " + c.mimeType : "") + "] " + (c.value !== undefined ? (typeof c.value === "string" ? c.value : JSON.stringify(c.value)) : c.data ? new TextDecoder().decode(c.data) : "")).join("\n");
    return "";
  }).join("");
}
function activate(context) {
  context.subscriptions.push(vscode.lm.registerLanguageModelChatProvider("spike", {
    async provideLanguageModelChatInformation() {
      return [{ id: "spike-model", name: "Spike model", family: "spike", version: "1", maxInputTokens: 100000, maxOutputTokens: 4000, capabilities: { toolCalling: true }, isUserSelectable: true, tooltip: "The spike scripted model", detail: "scripted" }];
    },
    async provideLanguageModelChatResponse(model, messages, options, progress) {
      const body = {
        model: "spike-model",
        messages: messages.map((m) => {
          const toolResult = m.content.find((p) => p instanceof vscode.LanguageModelToolResultPart);
          if (toolResult) return { role: "tool", tool_call_id: toolResult.callId, content: text(m) };
          return { role: m.role === vscode.LanguageModelChatMessageRole.User ? "user" : "assistant", content: text(m) };
        }),
        tools: (options.tools || []).map((t) => ({ type: "function", function: { name: t.name, description: t.description, parameters: t.inputSchema || {} } })),
      };
      const reply = await (await fetch(ENDPOINT, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) })).json();
      const message = reply.choices[0].message;
      for (const call of message.tool_calls || []) {
        progress.report(new vscode.LanguageModelToolCallPart(call.id, call.function.name, JSON.parse(call.function.arguments || "{}")));
      }
      if (message.content) progress.report(new vscode.LanguageModelTextPart(message.content));
    },
    async provideTokenCount(model, value) { return typeof value === "string" ? value.length : 100; },
  }));
}
module.exports = { activate };
