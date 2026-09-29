# Spec Delta

## MODIFIED Requirements

### Requirement: Requests are confined to the project

The tool SHALL restrict filesystem access to the project it is managing, SHALL restrict writes to the managed directories, and SHALL confine access to a specific post's directory when serving that post's assets. Containment checks SHALL reject symlink escapes, including dangling symlinks that could be followed by a write.

#### Scenario: Request escaping the project
- **WHEN** a request attempts to reach a path outside the project being managed
- **THEN** the tool refuses the request and discloses no information about the path

#### Scenario: Request escaping a post's directory
- **WHEN** a request for an asset escapes the directory of the post it belongs to, including by way of a symbolic link
- **THEN** the tool refuses the request
- **AND** the refusal is based on the resolved real path of the target, not the requested path

#### Scenario: Upload through a dangling symlink
- **WHEN** an asset upload would write through a dangling symlink to a path outside the post's directory or managed directories
- **THEN** the tool refuses the upload and creates or modifies no file outside the managed directories

#### Scenario: Asset request for a legitimate image
- **WHEN** a request names an image inside the post's own directory
- **THEN** the tool serves that image

## ADDED Requirements

### Requirement: Browser-launch failure is nonfatal

The tool SHALL continue serving its interface when it cannot start the platform's browser-launch command, and SHALL print the URL the user can open manually.

#### Scenario: Browser opener is unavailable
- **WHEN** the platform browser-launch command cannot be started
- **THEN** the tool prints the interface URL and continues serving without panicking or exiting
