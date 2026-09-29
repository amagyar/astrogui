# Spec Delta

## MODIFIED Requirements

### Requirement: Saves do not overwrite concurrent edits

The tool SHALL NOT save a post over changes made outside the editor after the post was opened unless the user explicitly chooses to replace those changes. On conflict, the tool SHALL make both the editor version and the current on-disk version available before the user chooses a recovery action. A retry SHALL still detect any newer external change.

#### Scenario: File changed since it was opened
- **WHEN** the user saves a post whose file changed on disk after the editor loaded it
- **THEN** the tool does not overwrite the on-disk change and reports the conflict with both versions available
- **AND** no version is discarded without the user choosing

#### Scenario: User explicitly keeps or merges the editor version
- **WHEN** a conflict is shown and the user reviews the on-disk version and explicitly chooses to save their retained or merged editor version
- **THEN** the tool saves against the version the user reviewed
- **AND** it reports another conflict rather than overwriting if the file changed again

#### Scenario: User chooses the on-disk version
- **WHEN** a conflict is shown and the user chooses to load the on-disk version
- **THEN** the editor loads the current file and its current modification token
- **AND** the prior editor version is discarded only after that explicit choice

## ADDED Requirements

### Requirement: Unsaved editor changes are protected

The tool SHALL warn before closing or replacing an editor that contains unsaved changes to the body, structured fields, or raw frontmatter, and SHALL offer an explicit way to keep editing, discard, or save those changes.

#### Scenario: User closes an editor with unsaved changes
- **WHEN** the user selects Board, presses Escape, or otherwise attempts to close an editor with unsaved changes
- **THEN** the tool asks whether to keep editing, discard, or save, and does not discard changes without an explicit choice

#### Scenario: User closes a clean editor
- **WHEN** the user closes an editor with no unsaved changes
- **THEN** the editor closes without an unnecessary warning

### Requirement: Created frontmatter preserves string values

The tool SHALL serialize user-provided frontmatter strings so that YAML-significant characters remain part of the original string value and cannot add or alter other fields.

#### Scenario: Title contains YAML-significant characters
- **WHEN** a post is created with a title containing characters such as `#`, `:`, quotes, or line breaks
- **THEN** the title round-trips as the same string and does not change the meaning or structure of other frontmatter

### Requirement: External images are not fetched implicitly

The preview SHALL NOT make network requests to external image hosts as a side effect of rendering post content, and SHALL clearly identify external images that it does not load.

#### Scenario: Post references an external image
- **WHEN** the editor preview encounters an HTTP, HTTPS, or protocol-relative image reference
- **THEN** it displays a clear external-image placeholder or notice without requesting the remote resource
- **AND** local images continue to use the token-checked asset path
