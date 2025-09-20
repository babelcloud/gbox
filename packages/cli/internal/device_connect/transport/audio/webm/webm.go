package webm

import (
	"time"

	"github.com/at-wat/ebml-go"
)

// EBMLHeader represents EBML header struct.
type EBMLHeader struct {
	EBMLVersion        uint64 `ebml:"EBMLVersion"`
	EBMLReadVersion    uint64 `ebml:"EBMLReadVersion"`
	EBMLMaxIDLength    uint64 `ebml:"EBMLMaxIDLength"`
	EBMLMaxSizeLength  uint64 `ebml:"EBMLMaxSizeLength"`
	DocType            string `ebml:"EBMLDocType"`
	DocTypeVersion     uint64 `ebml:"EBMLDocTypeVersion"`
	DocTypeReadVersion uint64 `ebml:"EBMLDocTypeReadVersion"`
}

// Seek represents Seek element struct.
type Seek struct {
	SeekID       []byte `ebml:"SeekID"`
	SeekPosition uint64 `ebml:"SeekPosition"`
}

// SeekHead represents SeekHead element struct.
type SeekHead struct {
	Seek []Seek `ebml:"Seek"`
}

// Info represents Info element struct.
type Info struct {
	TimecodeScale uint64    `ebml:"TimecodeScale"`
	MuxingApp     string    `ebml:"MuxingApp,omitempty"`
	WritingApp    string    `ebml:"WritingApp,omitempty"`
	Duration      float64   `ebml:"Duration,omitempty"`
	DateUTC       time.Time `ebml:"DateUTC,omitempty"`
}

// TrackEntry represents TrackEntry element struct.
type TrackEntry struct {
	Name            string `ebml:"Name,omitempty"`
	TrackNumber     uint64 `ebml:"TrackNumber"`
	TrackUID        uint64 `ebml:"TrackUID"`
	CodecID         string `ebml:"CodecID"`
	CodecPrivate    []byte `ebml:"CodecPrivate,omitempty"`
	CodecDelay      uint64 `ebml:"CodecDelay,omitempty"`
	TrackType       uint64 `ebml:"TrackType"`
	DefaultDuration uint64 `ebml:"DefaultDuration,omitempty"`
	SeekPreRoll     uint64 `ebml:"SeekPreRoll,omitempty"`
	Audio           *Audio `ebml:"Audio"`
	Video           *Video `ebml:"Video"`
}

// Audio represents Audio element struct.
type Audio struct {
	SamplingFrequency float64 `ebml:"SamplingFrequency"`
	Channels          uint64  `ebml:"Channels"`
}

// Video represents Video element struct.
type Video struct {
	PixelWidth  uint64 `ebml:"PixelWidth"`
	PixelHeight uint64 `ebml:"PixelHeight"`
}

// Tracks represents Tracks element struct.
type Tracks struct {
	TrackEntry []TrackEntry `ebml:"TrackEntry"`
}

// BlockGroup represents BlockGroup element struct.
type BlockGroup struct {
	BlockDuration  uint64     `ebml:"BlockDuration,omitempty"`
	ReferenceBlock int64      `ebml:"ReferenceBlock,omitempty"`
	Block          ebml.Block `ebml:"Block"`
}

// Cluster represents Cluster element struct.
type Cluster struct {
	Timecode    uint64       `ebml:"Timecode"`
	PrevSize    uint64       `ebml:"PrevSize,omitempty"`
	BlockGroup  []BlockGroup `ebml:"BlockGroup"`
	SimpleBlock []ebml.Block `ebml:"SimpleBlock"`
}

// Cues represents Cues element struct.
type Cues struct {
	CuePoint []CuePoint `ebml:"CuePoint"`
}

// CuePoint represents CuePoint element struct.
type CuePoint struct {
	CueTime           uint64             `ebml:"CueTime"`
	CueTrackPositions []CueTrackPosition `ebml:"CueTrackPositions"`
}

// CueTrackPosition represents CueTrackPosition element struct.
type CueTrackPosition struct {
	CueTrack           uint64 `ebml:"CueTrack"`
	CueClusterPosition uint64 `ebml:"CueClusterPosition"`
	CueBlockNumber     uint64 `ebml:"CueBlockNumber,omitempty"`
}

// Segment represents Segment element struct.
type Segment struct {
	SeekHead *SeekHead `ebml:"SeekHead"`
	Info     Info      `ebml:"Info"`
	Tracks   Tracks    `ebml:"Tracks"`
	Cluster  []Cluster `ebml:"Cluster"`
	Cues     *Cues     `ebml:"Cues"`
}

// SegmentStream represents Segment element struct for streaming.
type SegmentStream struct {
	SeekHead *SeekHead `ebml:"SeekHead"`
	Info     Info      `ebml:"Info"`
	Tracks   Tracks    `ebml:"Tracks"`
	Cluster  []Cluster `ebml:"Cluster,size=unknown"`
}
