# Spec Delta

## Purpose

Locates the Astro project astrogui is being run against, and defines the boundary that keeps every write the tool performs inside the user's own content directories.

## ADDED Requirements

### Requirement: Project location resolution

The tool SHALL identify the Astro project it is managing, and SHALL do so without modifying the project while doing so.

#### Scenario: Run from inside a recognized Astro project

- **WHEN** the user runs astrogui from within a directory that is an Astro project
- **THEN** the tool identifies that project and opens its board

#### Scenario: Run from a subdirectory of the project

- **WHEN** the user runs astrogui from a subdirectory of an Astro project
- **THEN** the tool resolves the project root by walking up to the nearest enclosing Astro project

#### Scenario: No Astro project found

- **WHEN** the user runs astrogui from a directory that is not inside an Astro project
- **THEN** the tool reports that no Astro project was found, names the directory it searched, and makes no changes to the filesystem

### Requirement: Content collection resolution

The tool SHALL determine which directory holds the blog's published content, and SHALL allow the user to correct that determination.

#### Scenario: Standard project layout

- **WHEN** the project's content collection is defined over the conventional content directory
- **THEN** the tool resolves the collection without user configuration

#### Scenario: User overrides the content location

- **WHEN** the user specifies a content directory that differs from the detected one
- **THEN** the tool uses the specified directory and persists the choice for subsequent runs

#### Scenario: Several collections are present

- **WHEN** the project defines more than one content collection
- **THEN** the tool asks which collection to manage and does not guess

### Requirement: Write boundary

The tool SHALL write only inside the directories it manages — the draft directories and the resolved content directory — and SHALL NOT write anywhere else in the project.

#### Scenario: Tool operation leaves unrelated project files untouched

- **WHEN** the user performs any operation in astrogui, including creating, editing, and publishing posts
- **THEN** every file the tool writes lies within the draft directories or the resolved content directory
- **AND** no file outside those directories is created, modified, or deleted

#### Scenario: Project configuration is never rewritten

- **WHEN** the user performs any operation in astrogui
- **THEN** the tool leaves the project's framework configuration, collection configuration, dependency manifest, and environment files unmodified

#### Scenario: The tool is not a project dependency

- **WHEN** the user installs or runs astrogui against a project
- **THEN** the project's dependency manifest and lockfile are unchanged
- **AND** the project builds and runs identically whether or not astrogui has ever been run against it

### Requirement: Published posts are never rewritten

The tool SHALL treat posts it did not create as read-only, and SHALL NOT restructure or reformat them.

#### Scenario: Project already contains loose markdown files

- **WHEN** the content directory already contains published posts stored as loose files rather than folders
- **THEN** the tool lists them on the board and offers editing only through the raw text view
- **AND** the tool does not move, migrate, or reformat them

#### Scenario: Publishing never overwrites an existing post

- **WHEN** a post is moved to the published state and a post already exists at the destination
- **THEN** the tool refuses the move, reports the conflict, and leaves both posts unchanged
