package id_test

import (
	"regexp"
	"testing"

	"github.com/babelcloud/gbox/packages/api-server/pkg/id"
	"github.com/stretchr/testify/assert"
)

func TestGenerateBoxID(t *testing.T) {
	// Test multiple IDs to ensure format and uniqueness
	generatedIDs := make(map[string]bool)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		boxID := id.GenerateBoxID()

		// Check format matches UUID v4: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		// where x is any hex digit, 4 is version, y is 8,9,a,b (variant bits)
		pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
		matched, err := regexp.MatchString(pattern, boxID)
		assert.NoError(t, err)
		assert.True(t, matched, "Generated ID %s does not match UUID v4 pattern", boxID)

		// Check uniqueness
		_, exists := generatedIDs[boxID]
		assert.False(t, exists, "Generated duplicate ID: %s", boxID)
		generatedIDs[boxID] = true

		// Check length is exactly 36 characters (32 hex + 4 hyphens)
		assert.Equal(t, 36, len(boxID), "UUID should be exactly 36 characters long")

		// Check that it has exactly 4 hyphens in the right positions
		assert.Equal(t, "-", string(boxID[8]), "Should have hyphen at position 8")
		assert.Equal(t, "-", string(boxID[13]), "Should have hyphen at position 13")
		assert.Equal(t, "-", string(boxID[18]), "Should have hyphen at position 18")
		assert.Equal(t, "-", string(boxID[23]), "Should have hyphen at position 23")

		// Check version bit (13th character should be '4')
		assert.Equal(t, "4", string(boxID[14]), "Version bit should be '4' for UUID v4")

		// Check variant bits (17th character should be 8, 9, a, or b)
		variantChar := string(boxID[19])
		assert.Contains(t, []string{"8", "9", "a", "b"}, variantChar,
			"Variant bit should be 8, 9, a, or b, got %s", variantChar)
	}
}
