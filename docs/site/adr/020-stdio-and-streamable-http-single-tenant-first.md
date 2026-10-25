---
search:
  boost: 0.3
---

# ADR-020: stdio and loopback Streamable HTTP from one server, for one person

`mm-mcp serve` runs the same server over stdio, for a client that starts it as a local process, or over Streamable HTTP with `--transport http`. Both serve one Mattermost server as one identity (ADR-019). Over HTTP the server authenticates no client, so anyone who reaches the port acts as that identity: it binds 127.0.0.1 by default and refuses an address that is not a loopback address.

mm-mcp is not run as a hosted, multi-user service, and its HTTP transport does not grow client authentication for one. The clients only a remote server reaches, those that cannot start a process, such as claude.ai and ChatGPT's connectors, speak the MCP authorization specification: the server is an OAuth 2.1 resource server whose metadata names an authorization server, and it accepts no token that was not issued for it. Every way to give mm-mcp one gives up what it is for, a server that needs no plugin and no administrator. Mattermost as the authorization server needs an administrator to turn on its OAuth service and let clients register, and the token it issues is Mattermost's, not bound to mm-mcp, so the remote client would hold a credential for the whole API. An authorization server of mm-mcp's own makes it a store of every user's Mattermost session, run in public by an operator. An organisation's OIDC provider leaves mm-mcp needing a system administrator's token to act as each user, since Mattermost exchanges no token and impersonates no one. An organisation that wants a central, administered server is better served by one that runs inside Mattermost.

Every tool reaches Mattermost through the `server.ClientFor` it was registered with, the one place that decides which identity a call acts as; the server passes `server.Single`, which answers with its one client. Reach Mattermost from a tool only through its ClientFor, never through a client held in a package variable or captured from elsewhere, and keep a feature that would hold state across calls with the identity, not the process. Do not lift the loopback restriction without client authentication in the same change, and a record that answers the paragraph above.

stdio is how desktop and IDE clients start a local server, and every client with a terminal or a desktop takes it; HTTP on a loopback address is for a client on the same machine that speaks only HTTP. An unauthenticated HTTP server bound to a network address hands the configured identity to the network.

## Not chosen

- **Hosted, multi-user operation**: Once the direction. Each way to authenticate a remote client needs an administrator, or a store of everyone's credentials and an operator to run it, and the clients it would reach are few; the people who use them can run mm-mcp on their own machine.
- **A single-user remote server**: One person runs mm-mcp where their phone's AI app can reach it, behind a minimal authorization server of mm-mcp's own whose login proves they are the configured identity. It needs no administrator, but it is security-critical code, run in public, for few people; it waits for someone to ask for it.
- **Mattermost's own OAuth, its token passed through**: The cheapest route, since Mattermost already registers clients and issues tokens with PKCE; but it needs the administrator mm-mcp exists to do without, and the MCP specification forbids a server accepting a token not issued for it.
- **A secret in the URL instead of authentication**: claude.ai and ChatGPT take any address, but a secret in a URL lands in logs and in the client vendor's configuration, and it would stand for a token that acts as a person.
- **stdio only**: A client on the same machine that speaks only HTTP would be left out, and the SDK serves HTTP from the same server with no second code path.
- **Bind any address and warn**: A warning in a log nobody reads is no protection for a token that acts as a person.
