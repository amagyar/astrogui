# Spec Delta

## MODIFIED Requirements

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
