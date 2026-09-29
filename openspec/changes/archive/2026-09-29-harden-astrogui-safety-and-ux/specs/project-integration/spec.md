# Spec Delta

## MODIFIED Requirements

### Requirement: Content collection resolution

The tool SHALL determine which directory holds the blog's published content, SHALL allow the user to correct that determination, and SHALL persist and honor a valid collection selection for subsequent runs. It SHALL NOT guess when multiple collections exist and no valid selection is configured.

#### Scenario: Standard project layout
- **WHEN** the project's content collection is defined over the conventional content directory
- **THEN** the tool resolves the collection without user configuration

#### Scenario: User overrides the content location
- **WHEN** the user specifies a content directory that differs from the detected one
- **THEN** the tool uses the specified directory and persists the choice for subsequent runs

#### Scenario: Several collections are present
- **WHEN** the project defines more than one content collection and no valid collection is configured
- **THEN** the tool asks which collection to manage and does not guess
- **AND** it persists the explicit selection for subsequent runs

#### Scenario: A valid collection is already configured
- **WHEN** the project defines more than one collection and a configured collection matches one of them
- **THEN** the tool manages that collection without prompting

#### Scenario: A configured collection is no longer present
- **WHEN** the configured collection does not match any detected collection
- **THEN** the tool asks the user to select a currently detected collection and persists the replacement choice
