# Spec Delta

## ADDED Requirements

### Requirement: A post can be renamed in place

The tool SHALL rename a folder post within its current state as a single atomic directory rename, keeping the post's contents unmodified and its pinned slug equal to the new name.

#### Scenario: Draft rename keeps identity consistent

- **WHEN** the user renames a folder post in a draft state
- **THEN** the folder is renamed atomically, the pinned `slug` frontmatter is updated to the new name, and no other content changes

#### Scenario: Rename to a taken name is refused

- **WHEN** the requested name already exists in the post's current state directory
- **THEN** the tool refuses the rename and leaves the post unchanged

#### Scenario: Renaming a published post warns about the URL

- **WHEN** the user renames a post in the published state
- **THEN** the tool requires an explicit confirmation that names the changed public URL before renaming

#### Scenario: Loose files are never renamed

- **WHEN** the user attempts to rename a loose post
- **THEN** the tool refuses, because posts the tool did not create are read-only

### Requirement: A post can be discarded to the trash

The tool SHALL provide a discard action that moves a folder post into a trash directory (`drafts/trash` unless configured otherwise) as a single atomic move, after explicit confirmation, and the trash directory SHALL NOT appear as board content.

#### Scenario: Discard removes the post from the board

- **WHEN** the user confirms discarding a folder post
- **THEN** the post's folder moves to the trash directory with its images intact, and no board column lists the trash directory

#### Scenario: Discard name collision

- **WHEN** the trash directory already holds a post of the same name
- **THEN** the discarded post is given a unique suffixed name rather than overwriting the earlier one

#### Scenario: Loose files are never discarded

- **WHEN** the user attempts to discard a loose post
- **THEN** the tool refuses, because posts the tool did not create are read-only

#### Scenario: Recovery is manual

- **WHEN** the user wants a discarded post back
- **THEN** moving its folder from the trash directory back into a draft directory with any tool restores it to the board, and astrogui performs no special bookkeeping for it
