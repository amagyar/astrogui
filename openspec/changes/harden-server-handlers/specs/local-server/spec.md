# Spec Delta

## MODIFIED Requirements

### Requirement: Requests are confined to the project

The tool SHALL restrict filesystem access to the project it is managing, SHALL restrict writes to the managed directories, and SHALL confine access to a specific post's directory when serving that post's assets. A post that has no directory of its own exposes no asset namespace at all. Containment checks SHALL reject symlink escapes, including dangling symlinks that could be followed by a write.

#### Scenario: Request escaping the project

- **WHEN** a request attempts to reach a path outside the project being managed
- **THEN** the tool refuses the request and discloses no information about the path

#### Scenario: Request escaping a post's directory

- **WHEN** a request for an asset escapes the directory of the post it belongs to, including by way of a symbolic link
- **THEN** the tool refuses the request
- **AND** the refusal is based on the resolved real path of the target, not the requested path

#### Scenario: Asset request for a legitimate image

- **WHEN** a request names an image inside the post's own directory
- **THEN** the tool serves that image

#### Scenario: Asset requests for a loose file are refused

- **WHEN** a request names an asset of a loose post (a bare markdown file, which has no directory of its own)
- **THEN** the tool refuses the request, exactly as asset upload refuses loose posts

#### Scenario: Served assets are inert documents

- **WHEN** an asset response is opened directly as a top-level document
- **THEN** response headers prevent it from executing script or embedding active content in the tool's origin, and from being sniffed into a different type

#### Scenario: Upload through a dangling symlink

- **WHEN** an asset upload would write through a dangling symlink to a path outside the post's directory or managed directories
- **THEN** the tool refuses the upload and creates or modifies no file outside the managed directories

## ADDED Requirements

### Requirement: Entry lookup errors are reported honestly

The API SHALL distinguish a post that does not exist from a post that exists but cannot be read, and SHALL NOT report an internal failure as a missing post.

#### Scenario: Post is absent

- **WHEN** a request names a post that is in no managed state
- **THEN** the API reports the post as not found

#### Scenario: Post exists but cannot be read

- **WHEN** a request names an existing post whose file cannot be read
- **THEN** the API reports a server error naming the read failure, not a not-found error

#### Scenario: Stat failure after a successful save

- **WHEN** a save succeeds but the fresh modification time cannot be read afterward
- **THEN** the save is still reported as successful, with the modification time unknown, and no handler panics
