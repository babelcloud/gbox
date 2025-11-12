package h264

import (
	"fmt"
	"io"
)

// AnnexBToAVCConverter converts H.264 Annex-B format to AVC format
type AnnexBToAVCConverter struct {
	buffer []byte
}

// NewAnnexBToAVCConverter creates a new converter
func NewAnnexBToAVCConverter() *AnnexBToAVCConverter {
	return &AnnexBToAVCConverter{
		buffer: make([]byte, 0, 1024*1024), // 1MB initial capacity
	}
}

// Convert converts H.264 Annex-B data to AVC format
// Annex-B uses 0x00000001 or 0x000001 as start codes
// AVC uses 4-byte length prefixes (big-endian)
func (c *AnnexBToAVCConverter) Convert(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Debug: Log first few conversions (can be removed in production)
	// if len(data) >= 4 {
	//	firstBytes := fmt.Sprintf("%02x %02x %02x %02x", data[0], data[1], data[2], data[3])
	//	if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
	//		fmt.Printf("[AnnexBToAVC] Converting Annex-B data (start code: %s), size: %d\n", firstBytes, len(data))
	//	} else {
	//		fmt.Printf("[AnnexBToAVC] Converting non-Annex-B data (first bytes: %s), size: %d\n", firstBytes, len(data))
	//	}
	// }

	// Reset buffer
	c.buffer = c.buffer[:0]

	// Find NAL units and convert them
	offset := 0
	for offset < len(data) {
		// Look for start code (0x00000001 or 0x000001)
		startCodePos := c.findStartCode(data[offset:])
		if startCodePos == -1 {
			// No more start codes found, add remaining data as last NAL unit
			if offset < len(data) {
				nalData := data[offset:]
				if len(nalData) > 0 {
					length := uint32(len(nalData))
					c.buffer = append(c.buffer,
						byte(length>>24),
						byte(length>>16),
						byte(length>>8),
						byte(length),
					)
					c.buffer = append(c.buffer, nalData...)
				}
			}
			break
		}

		// Calculate actual start code position
		actualPos := offset + startCodePos

		// If we have data before this start code, it's part of the previous NAL unit
		if actualPos > offset {
			nalData := data[offset:actualPos]
			length := uint32(len(nalData))
			c.buffer = append(c.buffer,
				byte(length>>24),
				byte(length>>16),
				byte(length>>8),
				byte(length),
			)
			c.buffer = append(c.buffer, nalData...)
		}

		// Skip the start code
		startCodeLen := c.getStartCodeLength(data[actualPos:])
		offset = actualPos + startCodeLen
	}

	return c.buffer, nil
}

// findStartCode finds the position of the next start code in the data
// Returns -1 if no start code is found
func (c *AnnexBToAVCConverter) findStartCode(data []byte) int {
	for i := 0; i < len(data)-3; i++ {
		// Check for 0x00000001
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			return i
		}
		// Check for 0x000001 (but not 0x00000001)
		if i < len(data)-2 && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			// Make sure it's not 0x00000001
			if i == 0 || data[i-1] != 0x00 {
				return i
			}
		}
	}
	return -1
}

// getStartCodeLength returns the length of the start code at the given position
func (c *AnnexBToAVCConverter) getStartCodeLength(data []byte) int {
	if len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
		return 4
	}
	if len(data) >= 3 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 {
		return 3
	}
	return 0
}

// ConvertStream converts a stream of H.264 Annex-B data to AVC format
func (c *AnnexBToAVCConverter) ConvertStream(input io.Reader, output io.Writer) error {
	buffer := make([]byte, 64*1024) // 64KB buffer
	var remaining []byte

	for {
		n, err := input.Read(buffer)
		if n > 0 {
			// Combine remaining data with new data
			data := append(remaining, buffer[:n]...)

			// Convert the data
			avcData, convertErr := c.Convert(data)
			if convertErr != nil {
				return fmt.Errorf("conversion error: %w", convertErr)
			}

			// Write converted data
			if len(avcData) > 0 {
				if _, writeErr := output.Write(avcData); writeErr != nil {
					return fmt.Errorf("write error: %w", writeErr)
				}
			}

			// Handle remaining data that might be part of an incomplete NAL unit
			remaining = c.getRemainingData(data)
		}

		if err == io.EOF {
			// Process any remaining data
			if len(remaining) > 0 {
				avcData, convertErr := c.Convert(remaining)
				if convertErr != nil {
					return fmt.Errorf("final conversion error: %w", convertErr)
				}
				if len(avcData) > 0 {
					if _, writeErr := output.Write(avcData); writeErr != nil {
						return fmt.Errorf("final write error: %w", writeErr)
					}
				}
			}
			break
		}

		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}

	return nil
}

// getRemainingData returns data that might be part of an incomplete NAL unit
func (c *AnnexBToAVCConverter) getRemainingData(data []byte) []byte {
	// Look for the last complete start code
	lastStartCode := -1
	for i := 0; i < len(data)-3; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			lastStartCode = i + 4
		} else if i < len(data)-2 && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			if i == 0 || data[i-1] != 0x00 {
				lastStartCode = i + 3
			}
		}
	}

	if lastStartCode == -1 {
		// No start code found, all data might be remaining
		return data
	}

	// Return data after the last start code
	if lastStartCode < len(data) {
		return data[lastStartCode:]
	}

	return nil
}

// ValidateAnnexBData validates that the data is in Annex-B format
func ValidateAnnexBData(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Check for start code at the beginning
	if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
		return true
	}
	if len(data) >= 3 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 {
		return true
	}

	return false
}

// ValidateAVCData validates that the data is in AVC format
func ValidateAVCData(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Check that it starts with a length prefix (not a start code)
	if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 && data[3] == 0x01 {
		return false // This is Annex-B format
	}
	if len(data) >= 3 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 {
		return false // This is Annex-B format
	}

	// Check for reasonable length prefix
	length := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if length > uint32(len(data)) {
		return false // Length exceeds available data
	}

	return true
}

// ConvertAnnexBToAVC is a convenience function for one-shot conversion
func ConvertAnnexBToAVC(data []byte) ([]byte, error) {
	converter := NewAnnexBToAVCConverter()
	return converter.Convert(data)
}

// PrependParameterSetsAVCC prepends SPS/PPS (Annex-B NAL payloads) to an AVCC-access unit
// sps, pps are raw NAL payloads (without start codes). Returns new AVCC buffer.
func PrependParameterSetsAVCC(avcc []byte, sps []byte, pps []byte) []byte {
	if len(avcc) == 0 || len(sps) == 0 || len(pps) == 0 {
		return avcc
	}
	// Build length-prefixed SPS and PPS
	spsLen := uint32(len(sps))
	ppsLen := uint32(len(pps))
	out := make([]byte, 0, 4+len(sps)+4+len(pps)+len(avcc))
	out = append(out, byte(spsLen>>24), byte(spsLen>>16), byte(spsLen>>8), byte(spsLen))
	out = append(out, sps...)
	out = append(out, byte(ppsLen>>24), byte(ppsLen>>16), byte(ppsLen>>8), byte(ppsLen))
	out = append(out, pps...)
	out = append(out, avcc...)
	return out
}

// ConvertAVCToAnnexB converts AVC format back to Annex-B format (for testing)
func ConvertAVCToAnnexB(data []byte) ([]byte, error) {
	var result []byte
	offset := 0

	for offset < len(data) {
		if offset+4 > len(data) {
			break // Not enough data for length prefix
		}

		// Read length prefix
		length := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3])
		offset += 4

		if offset+int(length) > len(data) {
			return nil, fmt.Errorf("invalid length prefix: %d", length)
		}

		// Add start code
		result = append(result, 0x00, 0x00, 0x00, 0x01)

		// Add NAL unit data
		result = append(result, data[offset:offset+int(length)]...)

		offset += int(length)
	}

	return result, nil
}
