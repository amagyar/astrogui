# Tasks

## 1. index.mdx folder posts

- [x] 1.1 In `internal/posts/posts.go` `listDir` and `internal/lifecycle` `FindIn`, recognize a folder post by `index.md` or `index.mdx` (prefer `index.md` when both exist, deterministic); update `Read`'s loose-file inference; verify new tests in `posts_test.go`/`lifecycle_test.go` list, open, and move an `index.mdx` folder post and `go test ./internal/posts ./internal/lifecycle` passes
- [x] 1.2 In `internal/project/project.go` `hasMarkdown`, count a folder with `index.mdx` as markdown content; verify with a `project_test.go` case that a legacy-layout directory holding only an `index.mdx` folder is detected
- [x] 1.3 Add an `index.mdx` folder post to `testdata/e2e-blog` and extend `scripts/e2e.sh` expectations if needed; verify `scripts/e2e.sh` passes

## 2. Script-independent names

- [x] 2.1 Rewrite `lifecycle.Slugify` to keep Unicode letters and numbers (`unicode.IsLetter`/`IsNumber`, lowercased) and normalize separator runs to `-`; verify new `lifecycle_test.go` cases: a Japanese title yields a Japanese name, an all-punctuation title yields the unique placeholder, existing ASCII slugs are unchanged
- [x] 2.2 Confirm the pinned-slug invariant end to end: create a post from a non-ASCII title via the API in `server_test.go` and assert frontmatter `slug:` equals the derived folder name
- [x] 2.3 Update `README.md`'s slug-pinning note only if the derivation behavior described there changes; verify wording matches implementation

## 3. Integration

- [x] 3.1 Run `go test ./...` and `node --test "tests/**/*.test.mjs"`; both pass
