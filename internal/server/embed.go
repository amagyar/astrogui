package server

import (
	"embed"
	"io/fs"
)

// The interface is embedded in the binary: no runtime dependency resolution,
// no module errors inside someone else's blog.
//
//go:embed web
var WebFS embed.FS

// UI returns the embedded interface rooted at the web directory.
func UI() fs.FS {
	sub, err := fs.Sub(WebFS, "web")
	if err != nil {
		panic(err) // embedded content: a failure here is a build error
	}
	return sub
}
