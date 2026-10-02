# Spec Delta

## MODIFIED Requirements

### Requirement: Board tracks the filesystem

The tool SHALL keep the board consistent with the filesystem while it is running, including changes the user makes outside the tool. Refresh SHALL be driven by change notifications from the managed directories; a slower periodic refresh SHALL take over when change notification is unavailable, so the board stays live either way.

#### Scenario: Post created outside the tool

- **WHEN** the user creates, renames, or deletes a post outside astrogui while it is running
- **THEN** the board reflects the change promptly after the filesystem reports it, without the user reloading

#### Scenario: Post moved outside the tool

- **WHEN** the user moves a post between directories outside astrogui while it is running
- **THEN** the post appears in the column matching its new location

#### Scenario: Change notification is advisory

- **WHEN** the board reports that a post changed outside the tool
- **THEN** the tool does not overwrite or revert that change

#### Scenario: Change notification unavailable

- **WHEN** filesystem change notification could not be established or has failed
- **THEN** the board still refreshes periodically, and the tool reports that it is polling instead

## ADDED Requirements

### Requirement: Board refreshes preserve user context

A refresh of the board SHALL NOT disturb an interaction in progress: keyboard focus, an open move control, an active drag, and column scroll position survive a refresh, and a refresh with no underlying change SHALL alter nothing.

#### Scenario: No-change refresh is invisible

- **WHEN** a refresh finds the same posts in the same states as the current display
- **THEN** the board's DOM is not rebuilt and nothing the user is doing is interrupted

#### Scenario: Refresh during keyboard navigation

- **WHEN** the user has tabbed to a card or control on the board and a refresh arrives with changes
- **THEN** the refresh is applied without moving keyboard focus away from the user's position, or it is deferred until the board is no longer in active use

#### Scenario: Refresh with an open move control or active drag

- **WHEN** a refresh arrives while a card's move control is open or a card is being dragged
- **THEN** the refresh is deferred until that interaction ends
- **AND** no pending move is silently cancelled by the refresh
