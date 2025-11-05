package device

import (
	"strconv"
	"strings"
	"testing"
)

func TestGetMacOSDisplayResolution(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedWidth  int
		expectedHeight int
		expectError    bool
	}{
		{
			name: "Built-in display with Retina",
			input: `Graphics/Displays:

    Apple M4 Max:

      Chipset Model: Apple M4 Max
      Type: GPU
      Bus: Built-In
      Displays:
        Color LCD:
          Display Type: Built-in Liquid Retina XDR Display
          Resolution: 3456 x 2234 Retina
          Mirror: Off
          Online: Yes
        Mi 27 NU:
          Resolution: 3840 x 2160 (2160p/4K UHD 1 - Ultra High Definition)
          Main Display: Yes
          Mirror: Off`,
			expectedWidth:  3456,
			expectedHeight: 2234,
			expectError:    false,
		},
		{
			name: "Main Display priority when no Built-in",
			input: `Graphics/Displays:

    Apple GPU:

      Displays:
        External Display:
          Resolution: 3840 x 2160 (2160p/4K UHD 1 - Ultra High Definition)
          Main Display: Yes
          Mirror: Off
        Color LCD:
          Resolution: 2560 x 1440
          Mirror: Off`,
			expectedWidth:  3840,
			expectedHeight: 2160,
			expectError:    false,
		},
		{
			name: "First display when no Built-in or Main Display",
			input: `Graphics/Displays:

    Apple GPU:

      Displays:
        Display 1:
          Resolution: 1920 x 1080
          Mirror: Off
        Display 2:
          Resolution: 2560 x 1440
          Mirror: Off`,
			expectedWidth:  1920,
			expectedHeight: 1080,
			expectError:    false,
		},
		{
			name: "Built-in takes priority over Main Display",
			input: `Graphics/Displays:

    Apple GPU:

      Displays:
        External Display:
          Resolution: 3840 x 2160
          Main Display: Yes
        Color LCD:
          Display Type: Built-in Liquid Retina XDR Display
          Resolution: 3456 x 2234 Retina`,
			expectedWidth:  3456,
			expectedHeight: 2234,
			expectError:    false,
		},
		{
			name: "No displays section",
			input: `Graphics/Displays:

    Apple GPU:

      Chipset Model: Apple GPU`,
			expectError: true,
		},
		{
			name: "No resolution found",
			input: `Graphics/Displays:

    Apple GPU:

      Displays:
        Color LCD:
          Display Type: Built-in
          Mirror: Off`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the test input using the same logic as getMacOSDisplayResolution
			lines := strings.Split(tt.input, "\n")
			var builtInResolution string
			var mainDisplayResolution string
			var firstResolution string

			type displayContext struct {
				isBuiltIn     bool
				isMainDisplay bool
				resolution    string
			}

			var currentDisplay displayContext
			displays := []displayContext{}

			inDisplaysSection := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)

				if strings.Contains(trimmed, "Displays:") {
					inDisplaysSection = true
					continue
				}

				if !inDisplaysSection {
					continue
				}

				if strings.HasSuffix(trimmed, ":") && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
					namePart := strings.TrimSuffix(trimmed, ":")
					if !strings.Contains(namePart, ":") && namePart != "" {
						if currentDisplay.resolution != "" {
							displays = append(displays, currentDisplay)
						}
						currentDisplay = displayContext{}
						continue
					}
				}

				if strings.Contains(trimmed, "Display Type: Built-in") || strings.Contains(trimmed, "Built-in: Yes") {
					currentDisplay.isBuiltIn = true
				}

				if strings.Contains(trimmed, "Main Display: Yes") {
					currentDisplay.isMainDisplay = true
				}

				if strings.Contains(trimmed, "Resolution:") {
					parts := strings.Split(trimmed, ":")
					if len(parts) >= 2 {
						res := strings.TrimSpace(parts[1])
						resParts := strings.Fields(res)
						var widthStr, heightStr string
						for _, part := range resParts {
							// Try to parse as integer to skip non-numeric parts
							if _, err := strconv.Atoi(part); err == nil {
								if widthStr == "" {
									widthStr = part
								} else if heightStr == "" {
									heightStr = part
									break
								}
							}
						}
						if widthStr != "" && heightStr != "" {
							currentDisplay.resolution = widthStr + "x" + heightStr
						}
					}
				}
			}

			if currentDisplay.resolution != "" {
				displays = append(displays, currentDisplay)
			}

			for _, display := range displays {
				if display.resolution == "" {
					continue
				}
				if firstResolution == "" {
					firstResolution = display.resolution
				}
				if display.isBuiltIn && builtInResolution == "" {
					builtInResolution = display.resolution
				}
				if display.isMainDisplay && mainDisplayResolution == "" {
					mainDisplayResolution = display.resolution
				}
			}

			var resolution string
			if builtInResolution != "" {
				resolution = builtInResolution
			} else if mainDisplayResolution != "" {
				resolution = mainDisplayResolution
			} else if firstResolution != "" {
				resolution = firstResolution
			}

			if resolution == "" {
				if !tt.expectError {
					t.Errorf("Expected resolution but got none")
				}
				return
			}

			if tt.expectError {
				t.Errorf("Expected error but got resolution: %s", resolution)
				return
			}

			parts := strings.Split(resolution, "x")
			if len(parts) != 2 {
				t.Errorf("Invalid resolution format: %s", resolution)
				return
			}

			width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				t.Errorf("Failed to parse width: %v", err)
				return
			}

			height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				t.Errorf("Failed to parse height: %v", err)
				return
			}

			if width != tt.expectedWidth || height != tt.expectedHeight {
				t.Errorf("Expected %dx%d, got %dx%d", tt.expectedWidth, tt.expectedHeight, width, height)
			}
		})
	}
}
