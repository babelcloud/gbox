package stream

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
)

// SpsPpsExtractor handles H.264 SPS/PPS parameter extraction with protocol abstraction
type SpsPpsExtractor struct {
	logger *slog.Logger
}

// NewSpsPpsExtractor creates a new SPS/PPS extractor
func NewSpsPpsExtractor(logger *slog.Logger) *SpsPpsExtractor {
	return &SpsPpsExtractor{
		logger: logger,
	}
}

// ExtractFromCache extracts SPS/PPS parameters from cached data
func (e *SpsPpsExtractor) ExtractFromCache(source *scrcpy.Source, deviceSerial string) ([]byte, []byte, error) {
	var sps, pps []byte
	var spsPpsExtracted bool

	// Poll cached SPS/PPS for a short time to avoid forcing keyframe/reset
	pollStart := time.Now()
	for !spsPpsExtracted && time.Since(pollStart) < 3*time.Second {
		spsPpsData := source.GetSpsPps()
		e.logger.Info("Checking for cached SPS/PPS", "device", deviceSerial, "cache_size", len(spsPpsData))

		if len(spsPpsData) > 0 {
			e.logger.Info("Processing cached SPS/PPS data", "device", deviceSerial, "size", len(spsPpsData), "first_byte", spsPpsData[0])

			// Extract SPS/PPS using protocol-specific logic
			extractedSps, extractedPps, err := e.extractSpsPps(spsPpsData, deviceSerial)
			if err == nil && extractedSps != nil && extractedPps != nil {
				sps, pps = extractedSps, extractedPps
				spsPpsExtracted = true
				e.logger.Info("SPS/PPS extracted from cache", "device", deviceSerial, "sps_size", len(sps), "pps_size", len(pps))
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !spsPpsExtracted {
		return nil, nil, fmt.Errorf("failed to extract SPS/PPS from cache within timeout")
	}

	return sps, pps, nil
}

// extractSpsPps handles protocol-specific SPS/PPS extraction
func (e *SpsPpsExtractor) extractSpsPps(data []byte, deviceSerial string) ([]byte, []byte, error) {
	var sps, pps []byte

	// Parse SPS/PPS from cached data, support both Annex-B and avcC
	if len(data) > 0 && data[0] == 0x01 {
		// avcC format
		e.logger.Info("Detected avcC format, parsing...", "device", deviceSerial)
		if ps, pp, ok := ParseAvccForSpsPps(data); ok {
			sps, pps = ps, pp
			e.logger.Info("SPS/PPS extracted from avcC cache", "device", deviceSerial, "sps_size", len(sps), "pps_size", len(pps))
			return sps, pps, nil
		} else {
			e.logger.Warn("Failed to parse avcC format", "device", deviceSerial)
		}
	}

	if len(sps) == 0 || len(pps) == 0 {
		// Try Annex-B split
		e.logger.Info("Trying Annex-B format", "device", deviceSerial)
		sps, pps = e.extractFromAnnexB(data, deviceSerial)
	}

	if len(sps) > 0 && len(pps) > 0 {
		return sps, pps, nil
	}

	return nil, nil, fmt.Errorf("could not extract SPS/PPS from data")
}

// extractFromAnnexB extracts SPS/PPS from Annex-B format data
func (e *SpsPpsExtractor) extractFromAnnexB(data []byte, deviceSerial string) ([]byte, []byte) {
	var sps, pps []byte

	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	parts := bytes.Split(data, startCode)

	for i := 1; i < len(parts); i++ {
		nal := parts[i]
		if len(nal) == 0 {
			continue
		}

		nalType := nal[0] & 0x1F
		switch nalType {
		case 7: // SPS
			sps = nal
			e.logger.Info("Found SPS from cache", "device", deviceSerial, "size", len(sps))
		case 8: // PPS
			pps = nal
			e.logger.Info("Found PPS from cache", "device", deviceSerial, "size", len(pps))
		}
	}

	return sps, pps
}
