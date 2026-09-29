# Spec Delta

## MODIFIED Requirements

### Requirement: Installable without a build step

The tool SHALL be installable as a prebuilt binary through a global install of the scoped npm package `@amagyar/astrogui`, without compilation or an install-time download on the user's machine.

#### Scenario: Global npm install
- **WHEN** the user globally installs `@amagyar/astrogui` on a supported platform and architecture
- **THEN** npm installs the matching prebuilt platform package and exposes the `astrogui` command without a build step or postinstall download

#### Scenario: Unsupported platform
- **WHEN** the tool is installed on a platform or architecture with no prebuilt binary
- **THEN** the install or first invocation reports that clearly rather than failing obscurely

#### Scenario: Runs on a machine without a project
- **WHEN** the tool is started in a directory that is not an Astro project
- **THEN** it reports that no project was found and exits without modifying anything
