package server

import (
	"context"

	"github.com/vriesdemichael/mm-mcp/internal/mattermost"
)

// Mattermost's web app shows some of what a post holds only where the server
// says it may, in the client configuration every logged-in client gets. A post
// carries what the web app would show of it, so mm-mcp reads the same settings.

// display is what the server lets its clients show of a post.
type display struct {
	// names and icons are whether an integration's post shows the name and
	// icon it chose in place of its author's.
	names, icons bool
	// priority is whether a post shows the label its author marked it with,
	// and acknowledgements whether one that asks for them shows who gave them.
	priority, acknowledgements bool
	// blocks is whether a post shows the blocks and cards integrations post,
	// the feature flag the newest releases turn on and the ESR does not have.
	blocks bool
}

// readDisplay reads what the server lets its clients show of a post. It is
// read with every page of posts, whatever they hold, so that every tool that
// describes posts is seen to read it (ADR-028). GetClientConfig.
func readDisplay(ctx context.Context, client *mattermost.Client) (display, error) {
	config, err := client.ClientConfig(ctx)
	if err != nil {
		return display{}, err
	}
	return display{
		names:            config["EnablePostUsernameOverride"] == "true",
		icons:            config["EnablePostIconOverride"] == "true",
		priority:         config["PostPriority"] == "true",
		acknowledgements: config["PostAcknowledgements"] == "true",
		blocks:           config["FeatureFlagMmBlocksEnabled"] == "true",
	}, nil
}
