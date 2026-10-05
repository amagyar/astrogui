# Spec Delta

## ADDED Requirements

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
