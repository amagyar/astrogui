# dev-server-preview Specification

## Purpose

Bridges the editor's style-oblivious preview to the blog's own dev server, opening the post at its real URL so authors can check true rendering one click away.

## Requirements

### Requirement: A post can be opened on the project's dev server

The editor SHALL offer an action that opens the post's public URL — the configured dev-server base URL combined with the post's pinned slug — in a new browser tab, from the same interaction whether the post is a draft or published.

#### Scenario: Open the edited post

- **WHEN** the user invokes the dev-server action in the editor
- **THEN** a new browser tab opens at the post's URL on the dev server, derived from the configured base URL and the post's slug

#### Scenario: Dev server unreachable

- **WHEN** the dev-server action is invoked and the configured base URL does not answer
- **THEN** the tool reports that the dev server appears to be down, names the URL it tried, and offers to open it anyway or to copy it — and never reports success for a page it did not load

#### Scenario: Loose posts use their filename

- **WHEN** the dev-server action is invoked on a loose post that has no pinned slug
- **THEN** the URL is derived from the post's filename, and the action remains available

### Requirement: The dev-server base URL is configurable

The tool SHALL default the dev-server base URL to Astro's conventional address and SHALL honor a per-project `devUrl` override in the tool's own config, alongside the existing directory overrides.

#### Scenario: Default address

- **WHEN** no `devUrl` is configured for the project
- **THEN** `http://localhost:4321` is used as the base URL

#### Scenario: Configured address

- **WHEN** a per-project `devUrl` is configured
- **THEN** the action builds post URLs from that base, including a custom port or a base path

#### Scenario: The tool never manages the dev server

- **WHEN** the dev server is not running
- **THEN** the tool reports it but does not start, stop, or install anything in the project
