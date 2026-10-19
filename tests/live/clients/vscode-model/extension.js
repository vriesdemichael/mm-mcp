// Test only (ADR-036): offers the client tests' scripted model as a chat model,
// so VS Code's chat calls mm-mcp's tools as a test scripts them, with no
// inference. The model's address comes from SCRIPTED_MODEL_URL, which the test
// starts VS Code with.
const vscode = require("vscode");

function asText(message) {
  return message.content
    .map((part) => {
      if (part instanceof vscode.LanguageModelTextPart) return part.value;
      if (part instanceof vscode.LanguageModelToolResultPart) {
        return part.content.map((c) => (typeof c.value === "string" ? c.value : JSON.stringify(c.value ?? ""))).join("\n");
      }
      return "";
    })
    .join("");
}

function activate(context) {
  const endpoint = (process.env.SCRIPTED_MODEL_URL || "http://127.0.0.1:18080") + "/v1/chat/completions";
  context.subscriptions.push(
    vscode.lm.registerLanguageModelChatProvider("scripted", {
      async provideLanguageModelChatInformation() {
        return [{
          id: "scripted-model", name: "Scripted model", family: "scripted", version: "1",
          // Room for every tool VS Code and a writing server offer, so VS Code
          // offers each rather than groups the model must open first.
          maxInputTokens: 1000000, maxOutputTokens: 4000, capabilities: { toolCalling: 1000 },
          isUserSelectable: true, tooltip: "The client tests' scripted model", detail: "scripted",
        }];
      },
      async provideLanguageModelChatResponse(model, messages, options, progress) {
        const body = {
          model: "scripted-model",
          messages: messages.map((m) => {
            const result = m.content.find((p) => p instanceof vscode.LanguageModelToolResultPart);
            if (result) return { role: "tool", tool_call_id: result.callId, content: asText(m) };
            return { role: m.role === vscode.LanguageModelChatMessageRole.User ? "user" : "assistant", content: asText(m) };
          }),
          tools: (options.tools || []).map((t) => ({ type: "function", function: { name: t.name, description: t.description, parameters: t.inputSchema || {} } })),
        };
        const reply = await (await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) })).json();
        const message = reply.choices[0].message;
        for (const call of message.tool_calls || []) {
          progress.report(new vscode.LanguageModelToolCallPart(call.id, call.function.name, JSON.parse(call.function.arguments || "{}")));
        }
        if (message.content) progress.report(new vscode.LanguageModelTextPart(message.content));
      },
      // About four characters a token, as models count.
      async provideTokenCount(model, value) {
        return typeof value === "string" ? Math.ceil(value.length / 4) : 100;
      },
    }),
  );
}

module.exports = { activate };
