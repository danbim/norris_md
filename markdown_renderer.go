package main

import (
	"github.com/microcosm-cc/bluemonday" // input sanitizer
	"github.com/russross/blackfriday"    // markdown renderer
)

type MarkdownRenderer struct {
}

func (m MarkdownRenderer) render(input []byte) []byte {
	unsafe := blackfriday.MarkdownCommon([]byte("# Hello World"))
	return bluemonday.UGCPolicy().SanitizeBytes(unsafe)
}
