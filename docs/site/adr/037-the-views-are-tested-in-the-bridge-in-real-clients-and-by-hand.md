---
search:
  boost: 0.3
---

# ADR-037: The views are tested in the bridge, in real clients, and by hand

mm-mcp's views (ADR-022) are tested at three levels, each where it can be relied on. A check that gates a merge involves no inference and no closed-source client.

1. **The page against the specification.** The view page runs in headless Chromium inside the official MCP Apps bridge, `@modelcontextprotocol/ext-apps/app-bridge`, the specification authors' own host side, with made-up data, on every pull request: the View Tests job, which CI Complete requires (`task test:views`). It holds the handshake, what reaches the page and what it sends, every press and what the page does with the answer, and that the page acts only on a click and never speaks for the person. A host of mm-mcp's own would share mm-mcp's reading of the specification, as a fake Mattermost would share its reading of the API (ADR-005).
2. **Real clients that can be driven.** Goose's desktop app and VS Code, at the versions the repository pins, draw the views against the live instance with a scripted model, on every pull request, and a daily run tries each client's newest release (ADR-036).
3. **Apps no machine can drive.** Claude desktop, Claude on the web, and other apps whose source is closed are checked by a person before a release that changes the views, with the Client check issue form, which asks what the client tests ask and keeps the app's MCP logs as the record. These apps offer no automation interface, take no model but their own, and need a signed-in, paid account; driving their screens would put inference and an account into CI. This level stays a check by hand.

What a model does with the tools is not measured. Whether a model calls `show` when the person asks to see something, or `propose_post` when they should have the last word, depends on the model. A measure worth having runs many models of many sizes, many times over, at a cost an open-source project does not carry. The server's instructions and each tool's description say what the tool is for. An organisation that deploys mm-mcp with a model of its choice can measure how that model uses them. ADR-014 says what the gates measure.

## Not chosen

- **Scheduled evals with one model through its API**: One model's choices say little about another's, and the people who run mm-mcp choose their own; a measure across models of every size costs more than this project carries.
- **Driving the closed-source apps' screens**: It needs inference and a paid account in CI, and breaks on every change of their interface.
- **Every level on every pull request**: The apps a person checks by hand are checked when the views change, before a release, rather than holding up every change.
