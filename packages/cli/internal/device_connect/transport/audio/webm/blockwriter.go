package webm

import (
	"io"
	"os"

	"github.com/at-wat/ebml-go/mkvcore"
)

// NewSimpleBlockWriter creates BlockWriteCloser for each track specified as tracks argument.
// Blocks will be written to WebM as EBML SimpleBlocks.
// Resultant WebM is written to given io.WriteCloser and will be closed automatically; don't close it by yourself.
// Frames written to each track must be sorted by their timestamp.
func NewSimpleBlockWriter(w0 io.WriteCloser, tracks []TrackEntry, opts ...mkvcore.BlockWriterOption) ([]BlockWriteCloser, error) {
	trackDesc := []mkvcore.TrackDescription{}
	for _, t := range tracks {
		trackDesc = append(trackDesc,
			mkvcore.TrackDescription{
				TrackNumber: t.TrackNumber,
				TrackEntry:  t,
			})
	}
	options := []mkvcore.BlockWriterOption{
		mkvcore.WithEBMLHeader(DefaultEBMLHeader),
		mkvcore.WithSegmentInfo(DefaultSegmentInfo),
		mkvcore.WithBlockInterceptor(DefaultBlockInterceptor),
	}
	options = append(options, opts...)
	ws, err := mkvcore.NewSimpleBlockWriter(w0, trackDesc, options...)
	webmWs := []BlockWriteCloser{}
	for _, w := range ws {
		webmWs = append(webmWs, BlockWriteCloser(w))
	}
	return webmWs, err
}

// NewSimpleWriter creates BlockWriteCloser for each track specified as tracks argument.
// Blocks will be written to WebM as EBML SimpleBlocks.
// Resultant WebM is written to given io.WriteCloser.
// io.WriteCloser will be closed automatically; don't close it by yourself.
//
// Deprecated: This is exposed to keep compatibility with the old version.
// Use NewSimpleBlockWriter instead.
func NewSimpleWriter(w0 io.WriteCloser, tracks []TrackEntry, opts ...mkvcore.BlockWriterOption) ([]*FrameWriter, error) {
	os.Stderr.WriteString(
		"Deprecated: You are using deprecated webm.NewSimpleWriter and *webm.blockWriter.\n" +
			"            Use webm.NewSimpleBlockWriter and webm.BlockWriteCloser interface instead.\n" +
			"            See https://godoc.org/github.com/at-wat/ebml-go to find out the latest API.\n",
	)
	ws, err := NewSimpleBlockWriter(w0, tracks, opts...)
	var ws2 []*FrameWriter
	for _, w := range ws {
		ws2 = append(ws2, &FrameWriter{w})
	}
	return ws2, err
}
