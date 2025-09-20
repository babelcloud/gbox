package webm

import (
	"github.com/at-wat/ebml-go/mkvcore"
)

var (
	// DefaultEBMLHeader is the default EBML header used by BlockWriter.
	DefaultEBMLHeader = &EBMLHeader{
		EBMLVersion:        1,
		EBMLReadVersion:    1,
		EBMLMaxIDLength:    4,
		EBMLMaxSizeLength:  8,
		DocType:            "webm",
		DocTypeVersion:     4, // May contain v4 elements,
		DocTypeReadVersion: 2, // and playable by parsing v2 elements.
	}
	// DefaultSegmentInfo is the default Segment.Info used by BlockWriter.
	DefaultSegmentInfo = &Info{
		TimecodeScale: 1000000, // 1ms
		MuxingApp:     "ebml-go.webm.BlockWriter",
		WritingApp:    "ebml-go.webm.BlockWriter",
	}
	// DefaultBlockInterceptor is the default BlockInterceptor used by BlockWriter.
	DefaultBlockInterceptor = mkvcore.MustBlockInterceptor(mkvcore.NewMultiTrackBlockSorter(mkvcore.WithMaxDelayedPackets(16), mkvcore.WithSortRule(mkvcore.BlockSorterDropOutdated)))
)
