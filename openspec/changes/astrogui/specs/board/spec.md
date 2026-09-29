# Spec Delta

## Purpose

Presents posts as a kanban board whose columns are the lifecycle directories, and surfaces the time-based signals that a directory listing alone cannot show.

## ADDED Requirements

### Requirement: Columns correspond to lifecycle directories

The tool SHALL render one board column per lifecycle directory, and SHALL derive the board's contents from the filesystem rather than from stored state.

#### Scenario: Board reflects the directories present

- **WHEN** the user opens the board
- **THEN** each column's cards are the posts present in that column's directory

#### Scenario: Board survives state stored elsewhere

- **WHEN** the tool is restarted
- **THEN** the board is reconstructed from the directories and is identical to the board before the restart

### Requirement: Cards carry time and size signals

The tool SHALL show, for each card, how long the post has existed, how recently it changed, and how large it is.

#### Scenario: Card shows age and size

- **WHEN** a post appears on the board
- **THEN** its card shows the time since the post first appeared, the time since it last changed, and its current length

#### Scenario: Freshly created post

- **WHEN** the user creates a post
- **THEN** its card reflects that the post was just created and just changed

#### Scenario: Post captured as a bare idea

- **WHEN** the user captures an idea as a single line of text
- **THEN** the idea is recorded without requiring any structured metadata to be supplied

### Requirement: Stalled work is identifiable

The tool SHALL make it possible to distinguish posts that are progressing from posts that have not changed in a long time.

#### Scenario: Stalled post is distinguishable

- **WHEN** a post has not changed for longer than a configured staleness threshold
- **THEN** its card is marked as stalled and shows how long it has been unchanged

#### Scenario: Threshold is configurable

- **WHEN** the user changes the staleness threshold
- **THEN** the board's stalled markings are recomputed against the new threshold without any change to the posts themselves

#### Scenario: Size distinguishes a note from a draft

- **WHEN** two posts in the same column differ substantially in length
- **THEN** their cards show enough size information for the difference to be apparent at a glance

### Requirement: Board tracks the filesystem

The tool SHALL keep the board consistent with the filesystem while it is running, including changes the user makes outside the tool.

#### Scenario: Post created outside the tool

- **WHEN** the user creates, renames, or deletes a post outside astrogui while it is running
- **THEN** the board reflects the change without the user reloading

#### Scenario: Post moved outside the tool

- **WHEN** the user moves a post between directories outside astrogui while it is running
- **THEN** the post appears in the column matching its new location

#### Scenario: Change notification is advisory

- **WHEN** the board reports that a post changed outside the tool
- **THEN** the tool does not overwrite or revert that change

### Requirement: Funnel history is derivable

The tool SHALL be able to report how many posts have reached each state, using information it derives itself rather than requiring the user to record it.

#### Scenario: Funnel is reported without user input

- **WHEN** the user views the funnel
- **THEN** the counts of posts that have reached each state are reported
- **AND** the user was not required to supply or maintain those counts

#### Scenario: Derived history is disposable

- **WHEN** the tool's derived data is deleted
- **THEN** the board is fully reconstructible from the filesystem, and no post is lost
