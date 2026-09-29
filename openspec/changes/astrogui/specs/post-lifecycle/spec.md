# Spec Delta

## Purpose

Tracks a post from idea to published using a directory-based state machine, and guarantees a post becomes visible to the blog only after it has been checked and found safe to publish.

## ADDED Requirements

### Requirement: Lifecycle state is the filesystem

The tool SHALL represent each lifecycle state as a distinct directory, and SHALL NOT store lifecycle state in a field inside the post.

#### Scenario: No lifecycle field in the post

- **WHEN** the tool writes or edits a post
- **THEN** the post's frontmatter contains no field that exists solely to record the tool's workflow state
- **AND** the post's state is fully derivable from the directory it occupies

#### Scenario: State is readable without the tool

- **WHEN** the user inspects the project with any other tool
- **THEN** the lifecycle state of every post is apparent from the directory layout alone

### Requirement: Transitions are atomic moves

The tool SHALL perform every lifecycle transition as a single atomic filesystem move that leaves the post's contents unmodified.

#### Scenario: Moving a post between states

- **WHEN** the user moves a post from one state to another
- **THEN** the tool performs the move as a single operation that either fully succeeds or leaves both locations unchanged
- **AND** the post's file contents are byte-for-byte identical before and after the move

#### Scenario: Move preserves version history

- **WHEN** a post is moved between states in a git repository
- **THEN** version control records the move as a rename, so the post's history remains continuous across states

#### Scenario: Interrupted move

- **WHEN** a move cannot complete
- **THEN** the post remains entirely in its original state and the tool reports the failure

### Requirement: Publish is gated by a pre-flight check

The tool SHALL verify a post before moving it to the published state, and SHALL refuse the move when the check fails.

#### Scenario: Post references a missing image

- **WHEN** a post being published references an image that does not exist in the post's directory
- **THEN** the tool refuses to publish, names the missing image and the referencing line, and leaves the post in its current state

#### Scenario: Post is missing required frontmatter

- **WHEN** a post being published has no title, or a publication date that does not parse, or an empty body
- **THEN** the tool refuses to publish, reports which condition failed, and leaves the post in its current state

#### Scenario: Post passes the check

- **WHEN** a post being published satisfies every check
- **THEN** the tool moves the post to the published state and nothing else

#### Scenario: Checks do not read the project's schema

- **WHEN** the tool performs pre-flight checks
- **THEN** the tool does not execute or import the project's collection configuration to make its determinations

### Requirement: Published posts keep a stable URL

The tool SHALL ensure a post's public URL is determined by the post's identity and does not change when the post is moved or when the surrounding directory layout changes.

#### Scenario: Folder-based post does not alter its URL

- **WHEN** a post is stored as a folder and published
- **THEN** its public URL is derived from its slug, not from the folder or filename path it happens to occupy

#### Scenario: Moving a post does not change its URL

- **WHEN** a post is moved between lifecycle states
- **THEN** the public URL of the post before and after the move is identical

### Requirement: Version control actions are explicit and separate

The tool SHALL keep the state move distinct from any version control operation, and SHALL perform a version control operation only when the user explicitly requests it.

#### Scenario: Publishing performs no commit

- **WHEN** the user publishes a post
- **THEN** the tool moves the post and performs no version control operation

#### Scenario: Explicit commit

- **WHEN** the user invokes the commit action
- **THEN** the tool stages the working tree and creates a commit, and the user may confirm the commit message before it is created

#### Scenario: Explicit commit and push

- **WHEN** the user invokes the commit and push action
- **THEN** the tool creates the commit and then pushes it to the tracked remote

#### Scenario: Several posts published before committing

- **WHEN** the user publishes several posts and then invokes the commit action once
- **THEN** all of them are included in the single commit

#### Scenario: Push failure is reported

- **WHEN** the user invokes the commit and push action and the push fails
- **THEN** the tool reports the failure and its output, and does not report the action as successful
