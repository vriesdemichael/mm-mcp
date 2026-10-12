# Spike #37: MCP Apps views in real clients

This directory and the `spike` build tag are a throwaway, and never merge.
They answer the questions in
[#37](https://github.com/vriesdemichael/mm-mcp/issues/37), client by client.
The findings are recorded on that issue, not here.

## What the spike adds

Built with `-tags spike`, mm-mcp gains a view and a few tools. It also logs
every request a client sends, with its parameters, to stderr and to the file
named by `SPIKE_LOG`.

| Name | Seen by | What it does |
|---|---|---|
| `ui://mm-mcp/spike` | the client | the view, from `internal/server/spike/page.html` |
| `spike_show_post` | model | reads a post and shows it in the view. `mode` is `full`, `line` or `none`; `auto` runs the view's checks without a press; `ask` with `auto` has the view start a question |
| `spike_press` | view only (`visibility: ["app"]`) | records a press: the token in `_meta` or in an argument, and a note. It is annotated as a write |
| `spike_token` | view only | hands the view a token outside the result the model reads |
| `spike_ask` | model | asks the person a question (elicitation) |
| `spike_write` | model | changes nothing; it is annotated as a write, to see whether the client asks before it |

The spike's tools break mm-mcp's rules on purpose: two are annotated as
writes, are offered by a read-only server, and ask nothing. So
`go test -tags spike` fails the governance tests that guard those rules. CI
never builds with the tag.

The markers show where data travels. The scripted model logs each one it receives:

- `SPIKE-META-`: in the result's `_meta` only.
- `SPIKE-SC-`: in `structuredContent` only.
- `SPIKE-TOKEN-`: the token.
- `SPIKE-CTX-`: made by the view and sent through `ui/update-model-context`.

## Build and seed

```bash
go build -tags spike -o .tmp/mm-mcp-spike ./cmd/mm-mcp
go build -tags spike -o .tmp/spike-model ./spike/model
```

The spike reads a post from a throwaway Mattermost: the local live-test stack.

1. On that stack, make a user, a personal access token for it, and a post.
   Give the post Markdown and an HTML tag, for example
   `The release moved to **Thursday**. <img src=x onerror=alert(1)> stays text.`
2. Put them in `.tmp/spike.env`, which is never committed:

   ```bash
   MM_URL=http://localhost:8065
   MM_TOKEN=<the token>
   POST_ID=<the post's id>
   ```

Never use a token for a real Mattermost: the spike's HTTP mode below puts it
on a public address.

## By machine

`harness/` drives each client through Playwright 1.56.1 (`npm ci` in
`harness/`). The clients are started with a remote debugging port and attached
over CDP, on a virtual display (Xvfb). The scripted model answers a user
message `SPIKE <tool> <json arguments>` with a call of that tool, so no
inference is involved:

```bash
.tmp/spike-model -log .tmp/spike-model.jsonl &
```

### The reference host

This is `basic-host` from `@modelcontextprotocol/ext-apps`, over Streamable
HTTP.

1. Start the spike with `serve --transport http --port 3101`.
2. Start `cors-relay.mjs 3001 3101`.
3. Serve `basic-host` with `SERVERS='["http://localhost:3001/mcp"]'`.
4. Run `reference-host.mjs <post id> <screenshot dir>`.

### Goose desktop

Use the `.deb` unpacked into `GOOSE_DIR`.

```bash
GOOSE_DIR=… GOOSE_HOME=… [GOOSE_MODE=approve|smart_approve] harness/goose-start.sh
node harness/goose-send.mjs <dir> <name> 'SPIKE spike_show_post {"post_id":"…","mode":"full","auto":true}'
node harness/goose-click.mjs <dir> <name> "Allow Once"
```

`goose-model-ask.mjs`, `goose-ask.mjs` and `goose-reopen.mjs` cover the
question started by the model, the question started by the view, and stored
conversations.

### VS Code

Use the linux-x64 archive unpacked into `VSCODE_DIR`. `vscode-model/` is an
extension that offers the scripted model as a chat model. Choose it in the chat's
model picker ("Chat: Manage Language Models" lists it).

```bash
VSCODE_DIR=… VSCODE_DATA=… [AUTO_APPROVE=false] harness/vscode-start.sh
node harness/vscode-onboard.mjs <dir>     # past the welcome, without signing in
node harness/vscode-mcp.mjs <dir>         # MCP: List Servers, start the spike
node harness/vscode-send.mjs <dir> <name> 'SPIKE spike_show_post {…}'
node harness/vscode-click.mjs <dir> <name> True   # answers the question
node harness/vscode-history.mjs <dir>     # reopens a stored chat
```

## By hand: Claude desktop (canary) and Claude on the web

These clients are closed source. Record the client's version, and keep
`SPIKE_LOG` and the client's own MCP log as the record:

- Claude desktop on macOS: `~/Library/Logs/Claude/mcp*.log`.
- Claude desktop on Windows: `%APPDATA%\Claude\logs\`.

### Setup

**Claude desktop.** Turn on the canary flag. Under Settings → Developer → Edit
Config, add:

```json
{ "mcpServers": { "mm-mcp-spike": { "command": "/path/to/mm-mcp/spike/harness/mm-mcp-spike.sh" } } }
```

**Claude on the web.** A custom connector is reached from Anthropic's cloud,
so it needs a public HTTPS address. mm-mcp's HTTP transport authenticates no
one, so anyone who has that address acts as the spike's user. Do it only
against the throwaway stack, and only for the length of the test:

1. Start the spike with `serve --transport http --port 8765`.
2. Open a temporary tunnel to `http://127.0.0.1:8765`, for example
   `cloudflared tunnel --url http://127.0.0.1:8765`.
3. Add `<tunnel address>/mcp` as a custom connector.
4. When done, stop the tunnel and remove the connector.

### The checklist

In a new conversation, have Claude make each call below. Then answer the
question that goes with it.

1. **Footprint.** Call `spike_show_post` with your post's id and mode `full`,
   then `line`, then `none`.
   - Screenshot each.
   - Does `none` leave a frame, a gap or a spinner? How tall is `line`?
2. **Capability.** Does `SPIKE_LOG` show `io.modelcontextprotocol/ui` with
   `text/html;profile=mcp-app`?
   - Look in the `received initialize` line, or, on protocol 2026-07-28, in
     the `_meta` of each request.
   - Is it there only with the canary flag on?
3. **What the model sees.** After a `full` call, ask: "Quote every string in
   that tool result that starts with SPIKE-, exactly, and say which field it
   was in."
   - `SPIKE-META-` quoted means `_meta` reached the model.
   - `SPIKE-SC-` quoted means `structuredContent` reached it.
   - Is the reply text (`Showed the person the post…`) there too?
4. **Calls a view starts.** Press "Press: token in _meta", "Press: token in
   argument" and "Token from an app-only tool".
   - The `spike_press:` lines in `SPIKE_LOG` say whether each token arrived.
   - `spike_press` is annotated as a write. Did Claude ask before the view's
     call?
5. **Asking.** Press "Ask (elicitation)".
   - Does a question appear, and where? Answer it.
   - `spike_ask: answered …` in `SPIKE_LOG` confirms the answer arrived.
   - As a control, have Claude call `spike_ask` itself.
6. **Reporting back.** Press "Update model context", then ask: "What did the
   spike view report?" Does Claude quote the `SPIKE-CTX-` value in the view's
   log?
7. **Stored conversations.** Reload the page, or restart the app, and reopen
   the conversation.
   - Does the view render again?
   - Does `SPIKE_LOG` show new `spike_press` calls? The view runs its code
     again if it does.

Two more:

- **App-only tools.** Ask Claude to call `spike_press` with note "model". It
  should say it has no such tool.
- **Writes the model makes.** Have Claude call `spike_write`. Is an approval
  asked?
