---
search:
  boost: 0.3
---

# ADR-036: Real clients run the views at pinned versions, with a scripted model

The views are tested in the clients people use, not only in the official MCP Apps bridge (ADR-022). Goose's desktop app and VS Code each start mm-mcp over stdio against the live instance, draw its views, and are driven as a person would drive them, on every pull request, in the Client Tests job that CI Complete requires. A client's behaviour is what the views depend on and what the specification leaves open: where a view's data reaches the model, whether a view's call is allowed, what a client does with the tool only views call. The spike (mm-mcp#37) found each of those by driving these two clients by machine, and these tests keep finding them.

- **Pinned versions.** `.github/tool-versions.env` pins each client, and `go run ./tools/clients install`, which `task test:clients` runs first, unpacks the pinned release under `.tmp/clients/`, checked against the SHA-256 its publisher gives: GitHub's digest of a Goose release asset, VS Code's update service's. A failure on a pull request is then the change's, never a client's new release. The tests find an installed client and fail naming the task when it is missing (ADR-009).
- **A scripted model.** Each client's model is `internal/scriptedmodel`, an OpenAI-compatible endpoint the test serves on the loopback address. A message `SCRIPT <tool> <json>` is answered with that tool's call, a tool's result with Done, so the client calls mm-mcp exactly as the test says, at no cost, the same way every time. Goose reaches it as an OpenAI provider; VS Code through a development extension, `tests/live/clients/vscode-model`, that offers it as a chat model. The model keeps every request, so a test sees which tools the client put in front of it.
- **Driven over CDP.** Each client starts with a remote debugging port, and `tests/live/clients/driver.mjs` types, clicks and reads through Playwright's `connectOverCDP`, one command a run, printing what it found as JSON. The Go tests, in `tests/live` under the `live` and `clients` tags, seed Mattermost, start the client with a configuration of its own, and decide what each answer means. Every wait is on a condition with a deadline. A failure keeps a screenshot and the logs of the client and of mm-mcp in `.tmp/clients-artifacts`, which CI uploads.
- **Each client set apart.** Goose takes its configuration, data and state from `GOOSE_PATH_ROOT`, and VS Code its settings and `mcp.json` from its own user data directory, both in the test's temporary folder, so a run never touches the person's own. mm-mcp is started through a wrapper that holds its configuration, since Goose's desktop app passes on no environment of its own. VS Code starts with `--password-store=basic`, since without a keyring it shows a native dialog no debugging client can see, and its MCP server waits on it.
- **Where it runs.** CI runs the tests on Linux under Xvfb, and `task test:clients` starts a virtual display there itself when none is set. Elsewhere the clients draw on the screen of whoever runs the task.

A client that changes what the views rely on fails a pull request only once its pin moves, in a change of its own.

## Not chosen

- **Inference from a real model**: It costs money on every pull request, needs an account and a secret that a pull request from a fork cannot have, and answers differently from one run to the next. What the model decides is measured on its own, on a schedule (mm-mcp#40).
- **The newest client on every pull request**: A client's release would fail every pull request open on the day it ships, for a reason none of them caused.
- **Launching the client through Playwright's Electron support**: Launching the packaged apps that way timed out in the spike; a debugging port works for both.
- **Driving the clients from Go over CDP**: A view lives in frames inside other frames, in other processes, which Playwright follows and a hand-written client would have to.
- **A stand-in client of mm-mcp's own**: It would share mm-mcp's reading of the specification, as a fake Mattermost would share its reading of the API (ADR-005); the official bridge is the reference, and real clients are what people run.
- **Checksums kept in the repository**: Each publisher gives the SHA-256 of what it ships, read over TLS from the same place as the version; a second copy to move with every pin would only fall out of step.
