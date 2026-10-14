package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vriesdemichael/mm-mcp/internal/config"
)

// spikeRegister adds the MCP Apps spike (#37) to a server built with
// -tags spike. The spike never merges; without the tag it adds nothing.
var spikeRegister = func(*mcp.Server, ClientFor, config.Config) {}
