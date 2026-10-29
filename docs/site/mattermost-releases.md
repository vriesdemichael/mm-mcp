# Supported Mattermost releases

mm-mcp supports every Mattermost release from the oldest Extended Support
Release that Mattermost still supports to the newest release
([ADR-025](adr/025-supported-mattermost-releases.md)). When Mattermost names a new
Extended Support Release, the previous one stays supported until Mattermost ends
its support.

| | Release | Tested with |
|---|---|---|
| Extended Support Release | 11.7 | 11.7.11, Team Edition |
| Newest release | 11.11 | 11.11.1, Team Edition |

The live suite runs against both ends on every change, and against the newest
patch of every minor release between them once a week. An older release may
work, but nothing checks it.

The tests run on the free Team Edition
([ADR-007](adr/007-the-live-instance-is-team-edition-in-docker.md)). mm-mcp works
with the licensed editions too, which serve the same API, but offers no tools
for features only they have, such as scheduled posts, guest accounts and LDAP
groups. The login is the exception: it logs in through single sign-on, SAML,
Entra ID or OpenID Connect, which are licensed features; that is tested by hand
([For administrators](administrators.md#what-is-tested)).

## Differences between releases

Each operation a tool calls is compared between the two ends, from each
release's own route table and specification
([ADR-027](adr/027-a-difference-between-supported-releases-is-found-from-mattermosts-own-files.md)).

| Operation | Tool | On the Extended Support Release |
|---|---|---|
| `CreatePost` | `create_post`, `dm`, `group_message` | No `silent` parameter, which the tool never sends. Nothing differs in use. |
| `DeleteDraft` | `delete_draft` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |
| `DeleteDraftForThread` | `delete_draft` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |
| `GetDrafts` | `propose_post`, `list_drafts`, `delete_draft` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |
| `UpsertDraft` | `propose_post` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |
| `GetTeamByName` | `get_team_info` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |
| `SearchPostsInAllTeams` | `search_posts` | Served, as 11.7's router shows, but missing from its specification. Nothing differs in use. |

## Differences in what a release shows

Some releases show more of a post with the same operations. A post carries
what the release it was read on shows of it, and mm-mcp reads which from the
client configuration Mattermost gives every logged-in client, as Mattermost's
own apps do.

| What | Setting | On the Extended Support Release |
|---|---|---|
| Blocks and cards integrations post, in Mattermost's own blocks, Slack's Block Kit or Adaptive Cards | The feature flag `MmBlocksEnabled`, on by default from 11.11 | Not shown: 11.7 has no such flag, and a post shows its message and message attachments alone, as Mattermost shows it there. Where the flag is on, a post's blocks take the place of its message attachments and of the preview of its link, as they do in Mattermost. |
| The color of a message attachment's bar | The feature flag `MmBlocksEnabled`, which draws attachments as blocks | Only `good`, `warning`, `danger` or a hex color: 11.7 draws no bar in any other, and mm-mcp leaves it out. Where the flag is on, a CSS color such as `red` or `rgb(220, 0, 0)` is kept as the integration wrote it. |
