# Spec Delta

## Purpose

Hosts the tool as a local binary with a filesystem-writing HTTP API, protects that API against cross-origin and rebinding attacks, and makes the binary installable through standard channels.

## ADDED Requirements

### Requirement: Local-only network exposure

The tool SHALL listen only on the loopback interface, so that it is not reachable from other machines on the network.

#### Scenario: Not reachable from the network

- **WHEN** the tool is running
- **THEN** its listening port accepts connections from the local machine only
- **AND** a request arriving from another host on the local network is not served

#### Scenario: Port selection

- **WHEN** the tool starts and its preferred port is already in use
- **THEN** it reports the conflict and either selects a free port or exits, and does not silently take over the port

### Requirement: Cross-origin requests are refused

The tool SHALL require proof that a request originates from the tool's own interface, and SHALL refuse requests that do not carry it.

#### Scenario: Request without a valid session token

- **WHEN** a request to a filesystem-affecting endpoint arrives without the session token issued for the current run
- **THEN** the tool refuses the request and performs no filesystem operation

#### Scenario: Page on another site attempts to reach the tool

- **WHEN** a page loaded from an unrelated website issues a request to the tool's listening port
- **THEN** the tool refuses the request, because the page cannot obtain the session token

#### Scenario: Token is not disclosed in referral data

- **WHEN** the tool opens its interface in a browser
- **THEN** the session token is carried in the URL fragment rather than the query string, and is not sent to any other host

### Requirement: Requests are confined to the project

The tool SHALL restrict filesystem access to the project it is managing, and SHALL confine access to a specific post's directory when serving that post's assets.

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

### Requirement: Command execution is limited to the user's own commands

The tool SHALL invoke only operations the user already performs themselves, and SHALL never alter project configuration or secrets.

#### Scenario: Configured action runs a known command

- **WHEN** the user invokes an action that runs a project command
- **THEN** the tool runs that command in the project and reports its result

#### Scenario: Secrets are never written

- **WHEN** the tool performs any operation
- **THEN** it does not read, write, or modify the project's environment or secret files

#### Scenario: Unavailable command is reported

- **WHEN** an invoked command is not available in the environment
- **THEN** the tool reports that plainly and does not report the action as successful

### Requirement: Installable through standard channels

The tool SHALL be installable as a prebuilt binary through a global npm install and through a Homebrew formula, without a compilation step on the user's machine.

#### Scenario: Global npm install

- **WHEN** the user installs the tool globally with npm on a supported platform and architecture
- **THEN** a prebuilt binary for that platform is installed and the tool runs without any build step

#### Scenario: Homebrew install

- **WHEN** the user installs the tool with Homebrew
- **THEN** a prebuilt binary is installed and the tool runs without any build step

#### Scenario: Unsupported platform

- **WHEN** the tool is installed on a platform or architecture with no prebuilt binary
- **THEN** the install reports that clearly rather than failing obscurely

#### Scenario: Runs on a machine without a project

- **WHEN** the tool is started in a directory that is not an Astro project
- **THEN** it reports that no project was found and exits without modifying anything
