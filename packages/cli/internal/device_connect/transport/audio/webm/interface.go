package webm

import (
	"github.com/at-wat/ebml-go/mkvcore"
)

// BlockWriter is a WebM block writer interface.
type BlockWriter interface {
	mkvcore.BlockWriter
}

// BlockReader is a WebM block reader interface.
type BlockReader interface {
	mkvcore.BlockReader
}

// BlockCloser is a WebM closer interface.
type BlockCloser interface {
	mkvcore.BlockCloser
}

// BlockWriteCloser groups Writer and Closer.
type BlockWriteCloser interface {
	mkvcore.BlockWriteCloser
}

// BlockReadCloser groups Reader and Closer.
type BlockReadCloser interface {
	mkvcore.BlockReadCloser
}

// FrameWriter is a backward compatibility wrapper of BlockWriteCloser.
//
// Deprecated: This is exposed to keep compatibility with the old version.
// Use BlockWriteCloser interface instead.
type FrameWriter struct {
	mkvcore.BlockWriteCloser
}
