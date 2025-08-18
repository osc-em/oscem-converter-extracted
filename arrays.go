package conversion

import (
	"regexp"
	"sort"
	"strings"
)

// Handles the conversion of dynamic array fields with [N] notation.
// It takes CSV mapping patterns that contain [N] placeholders and processes input data
// to create properly structured arrays in the result JSON.
//
// Parameters:
//   - result: The target map where processed arrays will be added
//   - dynamicFieldPatterns: CSV mapping rows containing [N] notation patterns
//   - input: The input data map with field names as keys and values as strings
func processDynamicArrayFields(result map[string]interface{}, dynamicFieldPatterns []csvextract, input map[string]string) {
	if len(dynamicFieldPatterns) == 0 {
		return
	}
	// Group patterns by their common prefixes (everything before [N])
	prefixGroups := make(map[string][]csvextract)
	for _, pattern := range dynamicFieldPatterns {
		fieldPattern := getFieldPattern(pattern)
		if fieldPattern != "" && strings.Contains(fieldPattern, "[N]") {
			prefix := strings.Split(fieldPattern, "[N]")[0]
			prefixGroups[prefix] = append(prefixGroups[prefix], pattern)
		}
	}
	inputs := groupArrayInputs(input, prefixGroups)
	if len(inputs) == 0 {
		return
	}
	processedArrays := processEachArrayType(inputs, dynamicFieldPatterns)

	// Add arrays to result
	for arrayPath, arrayData := range processedArrays {
		addItemsToArrayPath(result, arrayData, arrayPath)
	}
}

// Extracts the appropriate field pattern from a CSV mapping row, based on priority.
func getFieldPattern(row csvextract) string {
	if row.FromMDOC != "" {
		return row.FromMDOC
	}
	if row.OptionalsMDOC != "" {
		return row.OptionalsMDOC
	}
	return ""
}

// Organizes input data by array path and index for array processing.
// It takes patterns grouped by common prefixes and matches them against input data,
// creating a nested structure: arrayPath -> arrayIndex -> inputData.
// This allows different array types to be processed separately while maintaining
// proper indexing within each array.
//
// Parameters:
//   - input: The input data map with field names as keys
//   - prefixGroups: Groups of CSV patterns organized by common prefixes
//
// Returns:
//   - Nested structure organized by array path and index
func groupArrayInputs(input map[string]string, prefixGroups map[string][]csvextract) map[string]map[string]map[string]string {
	inputs := make(map[string]map[string]map[string]string)

	for _, patterns := range prefixGroups {
		// Get target array path from first pattern
		var targetArrayPath string
		if len(patterns) > 0 {
			arrayPath, arrayName := parseArrayPathFromOSCEM(patterns[0].OSCEM)
			targetArrayPath = strings.Join(append(arrayPath, arrayName), ".")
		}
		if targetArrayPath == "" {
			continue
		}

		// Process each pattern in this prefix group
		for _, pattern := range patterns {
			fieldPattern := getFieldPattern(pattern)
			if fieldPattern == "" {
				continue
			}
			regexPattern := convertPatternToRegex(fieldPattern)
			if regexPattern == "" {
				continue
			}
			regex := regexp.MustCompile(regexPattern)
			for inputKey, inputValue := range input {
				if matches := regex.FindStringSubmatch(inputKey); len(matches) >= 2 {
					arrayIndex := matches[1]

					// Initialize nested maps
					if inputs[targetArrayPath] == nil {
						inputs[targetArrayPath] = make(map[string]map[string]string)
					}
					if inputs[targetArrayPath][arrayIndex] == nil {
						inputs[targetArrayPath][arrayIndex] = make(map[string]string)
					}
					inputs[targetArrayPath][arrayIndex][inputKey] = inputValue
				}
			}
		}
	}

	return inputs
}

// Processes grouped array data and returns organized arrays.
// It takes the nested input structure organized by array path and index, processes
// each array element individually, and returns a map of complete arrays ready to
// be added to the result. Array indices are sorted to ensure consistent ordering.
//
// Parameters:
//   - inputs: Nested map structure: arrayPath -> arrayIndex -> inputData
//   - dynamicFieldPatterns: CSV mapping patterns for processing individual elements
//
// Returns:
//   - map[string][]interface{}: Map of array paths to their processed array data
func processEachArrayType(inputs map[string]map[string]map[string]string, dynamicFieldPatterns []csvextract) map[string][]interface{} {
	arrayResults := make(map[string][]interface{})

	for arrayPath, arrayIndices := range inputs {
		var arrayData []interface{}

		// Sort indices for consistent order
		sortedIndices := make([]string, 0, len(arrayIndices))
		for index := range arrayIndices {
			sortedIndices = append(sortedIndices, index)
		}
		sort.Strings(sortedIndices)

		// Process each array index
		for _, index := range sortedIndices {
			inputData := arrayIndices[index]
			processedElement := processSingleInput(inputData, dynamicFieldPatterns)
			if len(processedElement) > 0 {
				arrayData = append(arrayData, processedElement)
			}
		}

		if len(arrayData) > 0 {
			arrayResults[arrayPath] = arrayData
		}
	}

	return arrayResults
}

// Processes a single array element from input data.
// It takes input data for one array index and converts it into a structured
// object by matching field patterns, extracting property names, applying unit
// conversions, and creating nested structures as needed.
//
// Parameters:
//   - input: Input data for a single array element (one index)
//   - dynamicFieldPatterns: CSV mapping patterns containing [N] notation
//
// Returns:
//   - map[string]interface{}: Processed object representing one array element
func processSingleInput(input map[string]string, dynamicFieldPatterns []csvextract) map[string]interface{} {
	singleInput := make(map[string]interface{})

	for _, row := range dynamicFieldPatterns {
		if !strings.Contains(row.OSCEM, "[N]") {
			continue
		}
		fieldPattern := getFieldPattern(row)
		if fieldPattern == "" {
			continue
		}
		regexPattern := convertPatternToRegex(fieldPattern)
		if regexPattern == "" {
			continue
		}
		regex := regexp.MustCompile(regexPattern)

		for inputKey, inputValue := range input {
			if matches := regex.FindStringSubmatch(inputKey); len(matches) >= 2 {
				propertyName := extractPropertyName(row.OSCEM)
				if propertyName == "" {
					continue
				}
				// Apply unit conversion using priority-based crunch factor
				crunchFactor := getCrunchFactor(row)
				value := processValue(inputValue, crunchFactor, row)
				// Insert the value into the result structure
				if strings.Contains(propertyName, ".") {
					insertNested(singleInput, strings.Split(propertyName, "."), value)
				} else {
					singleInput[propertyName] = value
				}
				break
			}
		}
	}
	return singleInput
}

// Converts a field pattern with [N] notation to a regex pattern.
// It escapes special regex characters in the pattern and replaces [N] with a capture
// group that matches any sequence of non-dot characters.
// Example: "Detectors.Detector-[N].DetectorName" becomes "^Detectors\.Detector-([^.]+)\.DetectorName$"
//
// Parameters:
//   - fieldPattern: Field pattern string containing [N] notation
//
// Returns:
//   - string: Regex pattern with anchors, or empty string if no [N] found
func convertPatternToRegex(fieldPattern string) string {
	if !strings.Contains(fieldPattern, "[N]") {
		return ""
	}
	escaped := regexp.QuoteMeta(fieldPattern)
	regexPattern := strings.ReplaceAll(escaped, "\\[N\\]", "([^.]+)")
	return "^" + regexPattern + "$"
}

// Extracts the appropriate unit conversion factor from a CSV mapping row, following the same priority.
func getCrunchFactor(row csvextract) string {
	if row.FromMDOC != "" {
		return row.CrunchFromMDOC
	}
	if row.OptionalsMDOC != "" {
		return row.CrunchFromMDOC
	}
	if row.FromXML != "" {
		return row.CrunchFromXML
	}
	if row.OptionalsXML != "" {
		return row.CrunchFromXML
	}
	return ""
}

// Parses an OSCEM path to extract array path components.
// It splits the OSCEM path at the [N] notation to determine the parent path and array name.
// This is used to determine where in the result structure the processed array should be placed.
// Example: "acquisition.detectors[N].name" returns (["acquisition"], "detectors")
//
// Parameters:
//   - oscem: OSCEM path string containing [N] notation
//
// Returns:
//   - []string: Parent path components
//   - string: The name of the array property
func parseArrayPathFromOSCEM(oscem string) ([]string, string) {
	if !strings.Contains(oscem, "[N]") {
		return nil, ""
	}
	beforeArray := strings.Split(oscem, "[N]")[0]
	beforeParts := strings.Split(beforeArray, ".")
	if len(beforeParts) == 0 {
		return nil, ""
	}
	arrayName := beforeParts[len(beforeParts)-1]
	arrayParentPath := beforeParts[:len(beforeParts)-1]
	return arrayParentPath, arrayName
}

// Extracts the property name from an OSCEM path.
// It takes the part of the OSCEM path that comes after the [N] notation,
// which represents the property name within each array element.
// Leading dots are removed to get the clean property name.
// Example: "acquisition.detectors[N].name" returns "name"
//
// Parameters:
//   - oscem: OSCEM path string containing [N] notation
//
// Returns:
//   - string: Property name for the array element, or empty string if invalid format
func extractPropertyName(oscem string) string {
	parts := strings.Split(oscem, "[N]")
	if len(parts) == 2 {
		return strings.TrimPrefix(parts[1], ".")
	}
	return ""
}

// Adds processed array items to their target location in the result.
// It navigates through the nested result structure using the array path,
// and appends the new items to any existing array at the target location.
// This handles the final placement of processed arrays.
//
// Parameters:
//   - result: The target result map
//   - items: Array of processed items to add
//   - arrayPath: Dot-separated path indicating where to place the array (e.g., "acquisition.detectors")
func addItemsToArrayPath(result map[string]interface{}, items []interface{}, arrayPath string) {
	if len(items) == 0 {
		return
	}
	parts := strings.Split(arrayPath, ".")
	if len(parts) == 0 {
		return
	}

	// Navigate to the parent container
	parent := result
	for _, segment := range parts[:len(parts)-1] {
		if _, exists := parent[segment]; !exists {
			parent[segment] = make(map[string]interface{})
		}
		if nextParent, ok := parent[segment].(map[string]interface{}); ok {
			parent = nextParent
		} else {
			return
		}
	}

	// Get the array name (last part of the path)
	arrayName := parts[len(parts)-1]
	// Get existing array or create new one
	var existingArray []interface{}
	if existing, exists := parent[arrayName]; exists {
		if arr, ok := existing.([]interface{}); ok {
			existingArray = arr
		}
	}
	// Append the new items
	parent[arrayName] = append(existingArray, items...)
}
