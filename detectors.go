package conversion

import (
	"regexp"
	"sort"
	"strings"
)

func processDynamicDetectors(result map[string]interface{}, input map[string]string, rows []csvextract) {
	detectorInputs := groupDetectorInputs(input)
	if len(detectorInputs) == 0 {
		return
	}

	processedDetectors := processEachDetector(detectorInputs, rows)
	addDetectorsToResult(result, processedDetectors)
}

func groupDetectorInputs(input map[string]string) map[string]map[string]string {
	detectorRegex := regexp.MustCompile(`^Detectors\.Detector-([^.]+)\.(.+)$`)
	detectorInputs := make(map[string]map[string]string)

	for key, value := range input {
		if matches := detectorRegex.FindStringSubmatch(key); len(matches) == 3 {
			detectorID := matches[1]
			if detectorInputs[detectorID] == nil {
				detectorInputs[detectorID] = make(map[string]string)
			}
			detectorInputs[detectorID][key] = value
		}
	}
	return detectorInputs
}

func processEachDetector(detectorInputs map[string]map[string]string, rows []csvextract) []interface{} {
	var processedDetectors []interface{}

	// Sort for consistent output
	sortedIDs := make([]string, 0, len(detectorInputs))
	for id := range detectorInputs {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	for _, detectorID := range sortedIDs {
		detector := processSingleDetector(detectorInputs[detectorID], rows)
		if len(detector) > 0 {
			processedDetectors = append(processedDetectors, detector)
		}
	}
	return processedDetectors
}

func processSingleDetector(detectorInput map[string]string, rows []csvextract) map[string]interface{} {
	detector := make(map[string]interface{})

	for _, row := range rows {
		if !strings.Contains(row.OSCEM, "detectors[N]") {
			continue
		}

		// Create a closure that captures detectorInput for the extractor
		detectorExtractor := func(input map[string]string, fieldPattern string) ([]string, bool) {
			return extractDetectorValues(detectorInput, fieldPattern)
		}

		if rawValues, crunch, found := findMatchingValues(row, detectorInput, detectorExtractor); found {
			propertyName := extractPropertyName(row.OSCEM)
			value := processValue(rawValues[0], crunch, row)

			if strings.Contains(propertyName, ".") {
				insertNested(detector, strings.Split(propertyName, "."), value)
			} else {
				detector[propertyName] = value
			}
		}
	}
	return detector
}

func extractDetectorValues(detectorInput map[string]string, fieldPattern string) ([]string, bool) {
	if strings.Contains(fieldPattern, ";") {
		fieldNames := strings.Split(fieldPattern, ";")
		for _, fieldName := range fieldNames {
			fieldName = strings.TrimSpace(fieldName)
			if fieldName == "" {
				continue
			}

			// Check if any detector input matches this pattern
			for inputKey, inputValue := range detectorInput {
				if strings.HasSuffix(inputKey, "."+getLastPart(fieldName)) {
					return []string{inputValue}, true
				}
			}
		}
	} else {
		// Single field lookup
		for inputKey, inputValue := range detectorInput {
			if strings.HasSuffix(inputKey, "."+getLastPart(fieldPattern)) {
				return []string{inputValue}, true
			}
		}
	}
	return nil, false
}

func addDetectorsToResult(result map[string]interface{}, processedDetectors []interface{}) {
	if len(processedDetectors) == 0 {
		return
	}

	if _, exists := result["acquisition"]; !exists {
		result["acquisition"] = make(map[string]interface{})
	}
	acqMap := result["acquisition"].(map[string]interface{})

	var detArray []interface{}
	if existing, exists := acqMap["detectors"]; exists {
		if arr, ok := existing.([]interface{}); ok {
			detArray = arr
		}
	}
	acqMap["detectors"] = append(detArray, processedDetectors...)
}

func getLastPart(fieldName string) string {
	parts := strings.Split(fieldName, ".")
	return parts[len(parts)-1]
}

func extractPropertyName(oscem string) string {
	parts := strings.Split(oscem, "[N]")
	if len(parts) == 2 {
		return strings.TrimPrefix(parts[1], ".")
	}
	return ""
}
