# Spec Delta

## ADDED Requirements

### Requirement: The interface receives an advisory change feed

The local server SHALL offer the interface a stream of debounced filesystem change notifications for the managed directories, subject to the same origin and session-token checks as every other API. The stream is advisory: consumers re-read the filesystem rather than trusting event payloads.

#### Scenario: Interface subscribes to changes

- **WHEN** the interface opens the change feed with a valid session token
- **THEN** it receives a notification when a post is created, modified, moved, or deleted under a managed directory

#### Scenario: Feed obeys the same request controls

- **WHEN** a request to the change feed lacks a valid session token or carries an unexpected Host header
- **THEN** it is refused exactly as any other API request would be

#### Scenario: Feed failure degrades to polling

- **WHEN** the change feed is unavailable, disconnects, or could never be established
- **THEN** the interface continues with periodic refresh, and no user action is blocked by the feed's absence

#### Scenario: Events carry no content

- **WHEN** the feed reports a change
- **THEN** it carries only that something under a managed directory changed, and the interface obtains the new board state by re-reading the existing listing API
