package h264

import (
	"testing"
)

func TestAnnexBToAVCConversion(t *testing.T) {
	// Test data: SPS NAL unit in Annex-B format
	annexBData := []byte{
		0x00, 0x00, 0x00, 0x01, // Start code
		0x67, 0x42, 0x00, 0x1e, 0x96, 0x54, 0x05, 0x01, 0xed, 0x80, // SPS data
		0x00, 0x00, 0x00, 0x01, // Start code
		0x68, 0xce, 0x38, 0x80, // PPS data
	}

	converter := NewAnnexBToAVCConverter()
	avcData, err := converter.Convert(annexBData)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check that we got AVC format data
	if len(avcData) == 0 {
		t.Fatal("No AVC data returned")
	}

	// Verify format: should start with length prefix, not start code
	if avcData[0] == 0x00 && avcData[1] == 0x00 && avcData[2] == 0x00 && avcData[3] == 0x01 {
		t.Fatal("AVC data still contains start codes")
	}

	// Check that we have length prefixes
	if len(avcData) < 8 {
		t.Fatal("AVC data too short")
	}

	// First NAL unit length (SPS)
	spsLength := uint32(avcData[0])<<24 | uint32(avcData[1])<<16 | uint32(avcData[2])<<8 | uint32(avcData[3])
	if spsLength != 10 {
		t.Fatalf("Expected SPS length 10, got %d", spsLength)
	}

	// Second NAL unit length (PPS)
	ppsOffset := 4 + int(spsLength)
	if ppsOffset+4 > len(avcData) {
		t.Fatal("Not enough data for PPS length prefix")
	}
	ppsLength := uint32(avcData[ppsOffset])<<24 | uint32(avcData[ppsOffset+1])<<16 | uint32(avcData[ppsOffset+2])<<8 | uint32(avcData[ppsOffset+3])
	if ppsLength != 4 {
		t.Fatalf("Expected PPS length 4, got %d", ppsLength)
	}

	t.Logf("Successfully converted Annex-B to AVC: %d bytes -> %d bytes", len(annexBData), len(avcData))
}

func TestAVCToAnnexBConversion(t *testing.T) {
	// Test data: SPS and PPS in AVC format
	avcData := []byte{
		// SPS length (10 bytes) + SPS data
		0x00, 0x00, 0x00, 0x0a, // Length prefix
		0x67, 0x42, 0x00, 0x1e, 0x96, 0x54, 0x05, 0x01, 0xed, 0x80, // SPS data
		// PPS length (4 bytes) + PPS data
		0x00, 0x00, 0x00, 0x04, // Length prefix
		0x68, 0xce, 0x38, 0x80, // PPS data
	}

	annexBData, err := ConvertAVCToAnnexB(avcData)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check that we got Annex-B format data
	if len(annexBData) == 0 {
		t.Fatal("No Annex-B data returned")
	}

	// Verify format: should start with start code
	if !(annexBData[0] == 0x00 && annexBData[1] == 0x00 && annexBData[2] == 0x00 && annexBData[3] == 0x01) {
		t.Fatal("Annex-B data doesn't start with start code")
	}

	// Check that we have two NAL units
	startCodeCount := 0
	for i := 0; i < len(annexBData)-3; i++ {
		if annexBData[i] == 0x00 && annexBData[i+1] == 0x00 && annexBData[i+2] == 0x00 && annexBData[i+3] == 0x01 {
			startCodeCount++
		}
	}
	if startCodeCount != 2 {
		t.Fatalf("Expected 2 start codes, got %d", startCodeCount)
	}

	t.Logf("Successfully converted AVC to Annex-B: %d bytes -> %d bytes", len(avcData), len(annexBData))
}

func TestRoundTripConversion(t *testing.T) {
	// Test round-trip conversion: Annex-B -> AVC -> Annex-B
	originalAnnexB := []byte{
		0x00, 0x00, 0x00, 0x01, // Start code
		0x67, 0x42, 0x00, 0x1e, 0x96, 0x54, 0x05, 0x01, 0xed, 0x80, // SPS data
		0x00, 0x00, 0x00, 0x01, // Start code
		0x68, 0xce, 0x38, 0x80, // PPS data
	}

	// Convert to AVC
	converter := NewAnnexBToAVCConverter()
	avcData, err := converter.Convert(originalAnnexB)
	if err != nil {
		t.Fatalf("Annex-B to AVC conversion failed: %v", err)
	}

	// Convert back to Annex-B
	convertedAnnexB, err := ConvertAVCToAnnexB(avcData)
	if err != nil {
		t.Fatalf("AVC to Annex-B conversion failed: %v", err)
	}

	// Compare NAL unit data (excluding start codes)
	originalNals := extractNALUnits(originalAnnexB)
	convertedNals := extractNALUnits(convertedAnnexB)

	if len(originalNals) != len(convertedNals) {
		t.Fatalf("Different number of NAL units: original=%d, converted=%d", len(originalNals), len(convertedNals))
	}

	for i, originalNal := range originalNals {
		convertedNal := convertedNals[i]
		if len(originalNal) != len(convertedNal) {
			t.Fatalf("NAL unit %d length mismatch: original=%d, converted=%d", i, len(originalNal), len(convertedNal))
		}
		for j, b := range originalNal {
			if convertedNal[j] != b {
				t.Fatalf("NAL unit %d byte %d mismatch: original=0x%02x, converted=0x%02x", i, j, b, convertedNal[j])
			}
		}
	}

	t.Logf("Round-trip conversion successful: %d bytes -> %d bytes -> %d bytes", 
		len(originalAnnexB), len(avcData), len(convertedAnnexB))
}

func TestValidation(t *testing.T) {
	// Test Annex-B validation
	annexBData := []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42}
	if !ValidateAnnexBData(annexBData) {
		t.Error("Valid Annex-B data not recognized")
	}

	invalidData := []byte{0x01, 0x02, 0x03, 0x04}
	if ValidateAnnexBData(invalidData) {
		t.Error("Invalid data recognized as Annex-B")
	}

	// Test AVC validation
	avcData := []byte{0x00, 0x00, 0x00, 0x04, 0x67, 0x42, 0x00, 0x1e}
	if !ValidateAVCData(avcData) {
		t.Error("Valid AVC data not recognized")
	}

	if ValidateAVCData(annexBData) {
		t.Error("Annex-B data recognized as AVC")
	}
}

// Helper function to extract NAL units from Annex-B data
func extractNALUnits(data []byte) [][]byte {
	var nals [][]byte
	offset := 0

	for offset < len(data) {
		// Find start code
		startCodePos := -1
		for i := offset; i < len(data)-3; i++ {
			if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
				startCodePos = i
				break
			}
		}

		if startCodePos == -1 {
			break
		}

		// Skip start code
		startCodeLen := 4
		nalStart := startCodePos + startCodeLen

		// Find next start code
		nextStartCodePos := -1
		for i := nalStart; i < len(data)-3; i++ {
			if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
				nextStartCodePos = i
				break
			}
		}

		var nalEnd int
		if nextStartCodePos == -1 {
			nalEnd = len(data)
		} else {
			nalEnd = nextStartCodePos
		}

		// Extract NAL unit
		nal := make([]byte, nalEnd-nalStart)
		copy(nal, data[nalStart:nalEnd])
		nals = append(nals, nal)

		offset = nalEnd
	}

	return nals
}
