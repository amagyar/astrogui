# post-lifecycle Specification

## Purpose

Tracks a post from idea to published using a directory-based state machine, and guarantees a post becomes visible to the blog only after it has been checked and found safe to publish.

## Requirements

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

- **WHEN** a post with committed history is moved between states in a git repository
- **THEN** version control reports the move as a rename, so the post's history remains continuous across states
- **AND** a post that was never committed had no history to lose; its move simply presents new files at the new location

#### Scenario: Interrupted move

- **WHEN** a move cannot complete
- **THEN** the post remains entirely in its original state and the tool reports the failure

#### Scenario: Move across filesystems is refused

- **WHEN** a requested move has its source and destination on different filesystems, so no atomic rename exists
- **THEN** the tool refuses the move, names both locations, and leaves the post unchanged

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

The tool SHALL ensure a post's public URL is determined by the post's identity and is unaffected by the post's location in the pipeline.

#### Scenario: The URL derives from the pinned slug

- **WHEN** a post created by the tool is published
- **THEN** its entry id, and therefore its public URL, is derived from the `slug` the tool pinned at creation, not from the folder or file path it happens to occupy

#### Scenario: Draft-state moves cannot affect the URL

- **WHEN** a post is moved between draft states before publishing
- **THEN** no collection entry or public URL exists to be affected, because draft directories lie outside every collection and the move does not touch the slug

#### Scenario: The slug pin survives schema stripping

- **WHEN** the project's collection schema does not declare `slug`
- **THEN** the slug still pins the entry id at publish, because unknown frontmatter keys are stripped from entry data rather than rejected

### Requirement: Version control actions are explicit and separate

The tool SHALL keep the state move distinct from any version control operation, and SHALL perform a version control operation only when the user explicitly requests it. Before staging the working tree, the interface SHALL show the scope of the stage-all action and require explicit confirmation. A failed command SHALL expose its command and full output and SHALL NOT be reported as successful.

#### Scenario: Publishing performs no commit

- **WHEN** the user publishes a post
- **THEN** the tool moves the post and performs no version control operation

#### Scenario: Explicit commit

- **WHEN** the user invokes the commit action
- **THEN** the tool shows the files that the stage-all operation will include and explains that it stages the working tree
- **AND** only after explicit confirmation does it stage the working tree and create a commit with the user's confirmed message

#### Scenario: Explicit commit and push

- **WHEN** the user invokes the commit and push action
- **THEN** the tool shows the files that the stage-all operation will include and explains that it stages the working tree
- **AND** only after explicit confirmation does it create the commit and then push it to the tracked remote

#### Scenario: Several posts published before committing

- **WHEN** the user publishes several posts and then invokes the commit action once
- **THEN** all of them are included in the single commit

#### Scenario: Push failure is reported

- **WHEN** the user invokes the commit and push action and the push fails
- **THEN** the tool reports the failed command and its full output
- **AND** it does not report the action as successful

#### Scenario: Commit failure is reported

- **WHEN** the commit command fails
- **THEN** the tool reports the failed command and its full output
- **AND** it does not report the action as successful

#### Scenario: User cancels the stage-all confirmation

- **WHEN** the user reviews the stage-all scope and cancels
- **THEN** the tool performs no staging, commit, or push operation

### Requirement: A folder post is recognized by its index file

The tool SHALL treat a directory containing an `index.md` or `index.mdx` file as a folder post in every operation — listing, opening, moving, and pre-flight checks — and SHALL NOT silently skip it.

#### Scenario: Folder post written in MDX

- **WHEN** a managed directory contains a folder whose entry file is `index.mdx`
- **THEN** the folder appears on the board, can be opened, moved, and checked like an `index.md` folder

#### Scenario: Index file choice is preserved

- **WHEN** a folder post is edited through the tool
- **THEN** its original index filename (`index.md` or `index.mdx`) is preserved; the file form is never converted or duplicated

### Requirement: Post names carry the title's letters in any script

The tool SHALL derive a post's folder name from the letters and numbers of its title regardless of script, transliterating nothing but dropping only characters that are neither letters nor numbers, so titles in non-ASCII scripts do not collapse to a placeholder.

#### Scenario: Title in a non-ASCII script

- **WHEN** a post is created from a title whose letters are entirely non-ASCII (for example Japanese or Arabic)
- **THEN** the resulting folder name contains those letters and does not fall back to a placeholder

#### Scenario: Title with no usable letters

- **WHEN** a post is created from a title containing no letters or numbers in any script
- **THEN** the name falls back to a unique placeholder within the state directory

#### Scenario: Names stay URL-safe for Astro

- **WHEN** a name is derived from a title
- **THEN** it contains no characters that would make the resulting collection entry id invalid, because separator characters are normalized to `-` as before
