# Spec Delta

## ADDED Requirements

### Requirement: Board cards and lifecycle actions are keyboard accessible

The tool SHALL allow users to open a post and move it between lifecycle states without requiring a pointer or drag-and-drop.

#### Scenario: User opens a card with a keyboard
- **WHEN** a user navigates to a board card using the keyboard and activates it
- **THEN** the corresponding post editor opens

#### Scenario: User moves a card with a keyboard
- **WHEN** a user navigates to a card using the keyboard and chooses a destination state
- **THEN** the tool performs the same validated lifecycle transition as a drag-and-drop move

### Requirement: Board and editor adapt to narrow viewports

The tool SHALL keep board controls, cards, and editor content usable without horizontal page overflow on narrow viewports.

#### Scenario: Board on a narrow viewport
- **WHEN** the board is displayed on a narrow viewport
- **THEN** its columns reflow or become intentionally scrollable and all board controls remain reachable

#### Scenario: Editor on a narrow viewport
- **WHEN** the editor is displayed on a narrow viewport
- **THEN** source and preview remain readable and operable without forcing both panes into unusably narrow columns
