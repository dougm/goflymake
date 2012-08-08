## Emacs flymake-mode for the Go programming language

The `goflymake` program is a wrapper around the `go` tool to provide
Emacs flymake style syntax checking for golang source files within
multi-file packages and _test.go files.  Support for os/arch specific
*cgo* files is included thanks to the standard *go/build* package.

### Setup

 1. If needed, update your **$PATH** to include go installed binaries, for example:

    `export PATH=$PATH:$GOPATH/bin`

 2. Install goflymake:

    `go get -u github.com/dougm/goflymake`

### Emacs setup

 1. Install go-mode.el if you haven't already

 2. Add these lines to your **.emacs** or similar:

 		(add-to-list 'load-path "~/gocode/src/github.com/dougm/goflymake")
		(require 'go-flymake)

### ToDo

We probably shouldn't need the `goflymake` program, the `go` tool could
be tweaked to support the flymake style of syntax checking.
Maybe there is already a better way, but I couldn't find one.

