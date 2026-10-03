# Spec Delta

## ADDED Requirements

### Requirement: A titled post can be created from the board

The board SHALL offer a way to create a titled post directly in a chosen draft column, complementing one-line idea capture.

#### Scenario: Create with a title

- **WHEN** the user invokes the new-post action on a draft column and supplies a title
- **THEN** a folder post with that title is created in that column, and its card appears without a page reload

#### Scenario: Create requires a draft column

- **WHEN** the user attempts to create a post through the new-post action
- **THEN** only the draft columns offer the action; the published column does not

### Requirement: Card actions cover the post's own management

Each folder-post card SHALL expose its rename and discard actions alongside the existing move control, with loose-file cards showing them as unavailable.

#### Scenario: Card actions are reachable by keyboard

- **WHEN** the user focuses a card
- **THEN** open, move, rename, and discard are all operable without a pointer

#### Scenario: Destructive actions confirm inline

- **WHEN** the user chooses discard or the published-rename path from a card
- **THEN** a confirmation naming the consequence is shown before anything on disk changes
