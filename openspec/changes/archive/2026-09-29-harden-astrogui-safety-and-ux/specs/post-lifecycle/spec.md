# Spec Delta

## MODIFIED Requirements

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
- **WHEN** the user publishes several posts and then confirms the commit action once
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
