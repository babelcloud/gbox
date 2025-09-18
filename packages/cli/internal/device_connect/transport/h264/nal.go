package h264

import (
	"bytes"
)

var (
	// Standard Annex-B start codes
	StartCode3 = []byte{0x00, 0x00, 0x01}
	StartCode4 = []byte{0x00, 0x00, 0x00, 0x01}

	// AUD (Access Unit Delimiter) NAL unit - useful for some decoders
	AUDNalUnit = []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0x10}
)

// NALUnitType represents H.264 NAL unit types
type NALUnitType uint8

const (
	NALUnitTypeSlice     NALUnitType = 1
	NALUnitTypeDPA       NALUnitType = 2
	NALUnitTypeDPB       NALUnitType = 3
	NALUnitTypeDPC       NALUnitType = 4
	NALUnitTypeIDR       NALUnitType = 5
	NALUnitTypeSEI       NALUnitType = 6
	NALUnitTypeSPS       NALUnitType = 7
	NALUnitTypePPS       NALUnitType = 8
	NALUnitTypeAUD       NALUnitType = 9
	NALUnitTypeEndSeq    NALUnitType = 10
	NALUnitTypeEndStream NALUnitType = 11
	NALUnitTypeFiller    NALUnitType = 12
)

// GetNALUnitType extracts the NAL unit type from the first byte after start code
func GetNALUnitType(data []byte) (NALUnitType, bool) {
	nalStart := FindStartCode(data)
	if nalStart == -1 || nalStart+4 >= len(data) {
		return 0, false
	}

	// Skip start code and get NAL unit type from first 5 bits
	nalByte := data[nalStart+3] // For 3-byte start code, +4 for 4-byte
	if data[nalStart+1] == 0x00 && data[nalStart+2] == 0x00 && data[nalStart+3] == 0x01 {
		// 4-byte start code
		if nalStart+4 >= len(data) {
			return 0, false
		}
		nalByte = data[nalStart+4]
	}

	return NALUnitType(nalByte & 0x1F), true
}

// FindStartCode locates the position of the first start code in data
func FindStartCode(data []byte) int {
	if pos := bytes.Index(data, StartCode4); pos != -1 {
		return pos
	}
	if pos := bytes.Index(data, StartCode3); pos != -1 {
		return pos
	}
	return -1
}

// HasStartCode checks if data begins with a start code
func HasStartCode(data []byte) bool {
	return bytes.HasPrefix(data, StartCode4) || bytes.HasPrefix(data, StartCode3)
}

// AddStartCodeIfNeeded prepends a start code if the data doesn't already have one
func AddStartCodeIfNeeded(data []byte) []byte {
	if HasStartCode(data) {
		return data
	}

	// Use 4-byte start code by default
	result := make([]byte, 0, len(data)+4)
	result = append(result, StartCode4...)
	result = append(result, data...)
	return result
}

// ExtractSpsPpsAnnexB extracts SPS and PPS from Annex-B formatted data
// and returns them as separate NAL units with start codes
func ExtractSpsPpsAnnexB(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	// Add start code if needed
	processedData := AddStartCodeIfNeeded(data)

	// Split by start codes to find individual NAL units
	nalUnits := SplitByStartCodes(processedData)

	var result []byte
	for _, nalUnit := range nalUnits {
		if len(nalUnit) == 0 {
			continue
		}

		nalType, ok := GetNALUnitType(nalUnit)
		if !ok {
			continue
		}

		// Include SPS and PPS NAL units
		if nalType == NALUnitTypeSPS || nalType == NALUnitTypePPS {
			result = append(result, nalUnit...)
		}
	}

	return result
}

// SplitByStartCodes splits Annex-B data into individual NAL units,
// each retaining its start code
func SplitByStartCodes(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var nalUnits [][]byte
	var currentStart int

	// Find all start code positions
	for i := 0; i < len(data)-2; {
		// Look for 3-byte or 4-byte start codes
		if i < len(data)-3 && bytes.Equal(data[i:i+4], StartCode4) {
			// Found 4-byte start code
			if i > currentStart {
				nalUnits = append(nalUnits, data[currentStart:i])
			}
			currentStart = i
			i += 4
		} else if bytes.Equal(data[i:i+3], StartCode3) {
			// Found 3-byte start code
			if i > currentStart {
				nalUnits = append(nalUnits, data[currentStart:i])
			}
			currentStart = i
			i += 3
		} else {
			i++
		}
	}

	// Add the last NAL unit
	if currentStart < len(data) {
		nalUnits = append(nalUnits, data[currentStart:])
	}

	return nalUnits
}

// IsKeyFrame checks if the data contains an IDR (keyframe) NAL unit
func IsKeyFrame(data []byte) bool {
	nalUnits := SplitByStartCodes(data)
	for _, nalUnit := range nalUnits {
		if nalType, ok := GetNALUnitType(nalUnit); ok && nalType == NALUnitTypeIDR {
			return true
		}
	}
	return false
}

// PrependAUD adds an Access Unit Delimiter before the data.
// This can help some decoders properly parse frame boundaries.
func PrependAUD(data []byte) []byte {
	result := make([]byte, 0, len(AUDNalUnit)+len(data))
	result = append(result, AUDNalUnit...)
	result = append(result, data...)
	return result
}

// PrependSpsPps prepends SPS/PPS configuration data before keyframes.
// This ensures decoders have the necessary config data.
func PrependSpsPps(data []byte, spsPps []byte) []byte {
	if len(spsPps) == 0 {
		return data
	}

	result := make([]byte, 0, len(spsPps)+len(data))
	result = append(result, spsPps...)
	result = append(result, data...)
	return result
}

// StripEmulationPrevention removes emulation prevention bytes (0x03)
// from NAL unit data. This is sometimes needed for certain processing.
func StripEmulationPrevention(data []byte) []byte {
	if len(data) < 3 {
		return data
	}

	var result []byte
	for i := 0; i < len(data); {
		if i+2 < len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x03 {
			// Found emulation prevention sequence, skip the 0x03 byte
			result = append(result, data[i], data[i+1])
			i += 3
		} else {
			result = append(result, data[i])
			i++
		}
	}
	return result
}
