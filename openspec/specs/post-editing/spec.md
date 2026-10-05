# post-editing Specification

## Purpose

Lets the author write and edit a post in a split-pane editor while guaranteeing that content the tool did not intend to change is never altered.

## Requirements

### Requirement: Post body is edited verbatim

The tool SHALL present the post's markdown body as the post's actual content, and SHALL NOT transform it through an intermediate document representation.

#### Scenario: Specialized syntax survives editing

- **WHEN** a post contains fenced code blocks with language and display metadata, raw HTML, component tags, footnotes, tables, or trailing spaces used as line breaks
- **THEN** opening, editing, and saving the post leaves all of that syntax exactly as the author wrote it

#### Scenario: Save without a content change

- **WHEN** the user saves a post without having changed its body
- **THEN** the file's contents are unchanged

#### Scenario: Unchanged file is not rewritten

- **WHEN** the tool determines that a post requires no change
- **THEN** the tool does not rewrite the file, so its modification time is unchanged

### Requirement: Saves do not overwrite concurrent edits

The tool SHALL NOT save a post over changes made outside the editor after the post was opened unless the user explicitly chooses to replace those changes. On conflict, the tool SHALL make both the editor version and the current on-disk version available before the user chooses a recovery action. A retry SHALL still detect any newer external change.

#### Scenario: File changed since it was opened

- **WHEN** the user saves a post whose file changed on disk after the editor loaded it
- **THEN** the tool does not overwrite the on-disk change, and reports the conflict with both versions available
- **AND** no version is discarded without the user choosing

#### Scenario: User explicitly keeps or merges the editor version

- **WHEN** a conflict is shown and the user reviews the on-disk version and explicitly chooses to save their retained or merged editor version
- **THEN** the tool saves against the version the user reviewed
- **AND** it reports another conflict rather than overwriting if the file changed again

#### Scenario: User chooses the on-disk version

- **WHEN** a conflict is shown and the user chooses to load the on-disk version
- **THEN** the editor loads the current file and its current modification token
- **AND** the prior editor version is discarded only after that explicit choice

### Requirement: Rendered preview reflects the body

The tool SHALL show a rendered reading copy of the post alongside the source, and SHALL keep it in step with the text being edited, without performing redundant work whose cost grows with time spent typing.

#### Scenario: Preview updates while typing

- **WHEN** the user types in the source pane
- **THEN** the rendered pane reflects the new text without an explicit refresh step
- **AND** a pause in typing renders at most once per pause, not once per keystroke

#### Scenario: Inline code renders literally

- **WHEN** a code span (backtick-quoted text) contains emphasis markers or image syntax
- **THEN** the rendered pane shows those characters as literal code, not as formatting

#### Scenario: Local images render in the preview

- **WHEN** the post references an image that exists inside the post's directory
- **THEN** the rendered pane displays that image

#### Scenario: Local images are fetched once per editing session

- **WHEN** the same local image is still referenced after many preview updates
- **THEN** the preview reuses the previously fetched image instead of requesting it again
- **AND** fetched image resources are released when the editor closes

#### Scenario: Preview keeps the reader's place

- **WHEN** a preview update occurs while the user is reading the rendered pane
- **THEN** the rendered pane does not jump back to the top

#### Scenario: Missing image is visible in the preview

- **WHEN** the post references an image that does not exist
- **THEN** the rendered pane shows a visible indication of the broken reference

#### Scenario: Preview conveys content, not site styling

- **WHEN** the rendered pane is displayed
- **THEN** it reflects the post's content, and it does not claim to reproduce the published site's styling, code highlighting, or image optimization

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

### Requirement: The preview does not execute post content

The tool SHALL ensure that script content embedded in a post cannot execute in the tool's interface.

#### Scenario: Inline script in a post body

- **WHEN** a post body contains an inline `<script>` element or an HTML event-handler attribute
- **THEN** the rendered preview displays the post without executing that script

#### Scenario: Post content cannot reach the session token

- **WHEN** markup from a post is rendered in the preview
- **THEN** it cannot obtain the session token or use it to call the tool's API

### Requirement: Structured fields are edited separately from the body

The tool SHALL offer editable fields for common post metadata, and SHALL provide a way to view and edit the underlying metadata as text.

#### Scenario: Field editing updates the post

- **WHEN** the user changes a metadata field and saves
- **THEN** the post's metadata reflects the new value

#### Scenario: Fields the tool does not recognise are preserved

- **WHEN** the post's metadata contains fields the tool does not recognise, including comments and custom fields
- **THEN** editing a recognised field leaves those unrecognised fields and comments intact

#### Scenario: Raw metadata is editable

- **WHEN** the user opens the raw metadata view and edits it directly
- **THEN** the post's metadata reflects the edit

#### Scenario: Field is written only when changed

- **WHEN** the user saves without changing any metadata field
- **THEN** the tool does not rewrite the post's metadata

### Requirement: Images are managed with the post

The tool SHALL store images a user adds inside the post's own directory, and SHALL reference them relatively so they continue to resolve after the post is published.

#### Scenario: Pasted image is stored in the post

- **WHEN** the user pastes an image while editing a post
- **THEN** the image file is written into that post's directory and a reference to it is inserted into the body at the insertion point

#### Scenario: Reference is relative

- **WHEN** an image reference is inserted
- **THEN** the reference resolves relative to the post's file, and does not depend on the post's current location in the project

#### Scenario: Image pasted before the post has a name

- **WHEN** the user pastes an image into a post that has not been given a name yet
- **THEN** the tool gives the post and its image a usable name rather than failing

#### Scenario: Asset is not duplicated

- **WHEN** a post is published
- **THEN** its images move with it and are not left behind or copied into a second location
