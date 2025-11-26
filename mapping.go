package conversion

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/osc-em/oscem-converter-extracted/basetypes"
)

// Global storage for dynamic field patterns that weren't found in input and contain [N] notation.
var dynamicFieldPatterns []csvextract

func convertToHierarchicalJSON(rows []csvextract, input map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Clear any previously stored dynamic field patterns
	dynamicFieldPatterns = nil
	// Process regular mappings first - these handle direct field-to-field mappings
	processRegularMappings(result, rows, input)
	// Then process dynamic array fields - these handle patterns like [N]
	processDynamicArrayFields(result, dynamicFieldPatterns, input)

	return result, nil
}

// Handles standard field-to-field mappings from the CSV configuration.
// It iterates through each mapping rule and attempts to find matching values in the input data,
// then places them in the appropriate location in the output structure.
//
// Parameters:
//   - result: The output map being built
//   - rows: CSV mapping rules
//   - input: Source data as key-value pairs
func processRegularMappings(result map[string]interface{}, rows []csvextract, input map[string]string) {
	for _, row := range rows {
		// Try to find a matching value in the input data
		rawValues, crunchFactor, found := findMatchingValues(row, input, extractValuesFromInput)
		if !found {
			continue
		}
		// Determine if this is an array field (contains [N] notation) or regular field
		if strings.Contains(row.OSCEM, "[N]") {
			handleArrayField(result, row, rawValues, crunchFactor)
		} else {
			handleRegularField(result, row, rawValues, crunchFactor)
		}
	}
}

// A function type that defines how to extract values from input data.
type ValueExtractor func(csvextract, map[string]string, string) ([]string, bool)

// Searches for input data that matches a CSV mapping row using a priority system.
// The first matching field found is used, along with its corresponding unit conversion factor.
//
// Parameters:
//   - row: CSV mapping rules
//   - input: Source data as key-value pairs
//   - extractor: Function that defines how to extract values from the input
//
// Returns:
//   - []string: Array of values found
//   - string: Unit conversion factor to apply
//   - bool: Whether any matching values were found
func findMatchingValues(row csvextract, input map[string]string, extractor ValueExtractor) ([]string, string, bool) {
	// Priority order: optionals_mdoc > frommdoc > optionals_xml > fromxml
	checks := []struct {
		field  string
		crunch string
	}{
		{row.OptionalsMDOC, row.CrunchFromMDOC},
		{row.FromMDOC, row.CrunchFromMDOC},
		{row.OptionalsXML, row.CrunchFromXML},
		{row.FromXML, row.CrunchFromXML},
	}

	for _, check := range checks {
		if check.field != "" {
			if values, found := extractor(row, input, check.field); found {
				return values, check.crunch, true
			}
		}
	}
	return nil, "", false
}

// Extracts values from input data based on field patterns.
// It supports both single field lookups and semicolon-separated field lists.
// For semicolon-separated lists, it returns values in the same order as the field list,
// with empty strings for missing fields to maintain alignment.
// When field names contain [N] notation and don't exist in input, they are stored separately.
//
// Parameters:
//   - input: Source data as key-value pairs
//   - key: Field pattern to search for
//
// Returns:
//   - []string: Array of values found
//   - bool: Whether any matching values were found
func extractValuesFromInput(row csvextract, input map[string]string, key string) ([]string, bool) {
	if strings.Contains(key, ";") {
		// Handle semicolon-separated field names (e.g., "field1;field2;field3")
		fieldNames := strings.Split(key, ";")
		var result []string
		var foundAny bool

		// Iterate over each field name in the semicolon-separated list
		for _, fieldName := range fieldNames {
			fieldName = strings.TrimSpace(fieldName)
			if fieldName == "" {
				// If the field name is empty, append an empty string to maintain alignment
				result = append(result, "")
				continue
			}
			// If the field exists in the input, append its value and mark that at least one was found
			if val, exists := input[fieldName]; exists {
				result = append(result, val)
				foundAny = true
			} else {
				storeUnmappedField(row, fieldName)
				// Append an empty string to maintain alignment
				result = append(result, "")
			}
		}
		return result, foundAny
	} else {
		// Handle single field name lookup
		if val, exists := input[key]; exists {
			return []string{val}, true
		} else {
			storeUnmappedField(row, key)
			return nil, false
		}
	}
}

// Stores field names that contain [N] notation and weren't found in the input data,
// to be processed in the next step.
//
// Parameters:
//   - fieldName: The field name that wasn't found in the input data
func storeUnmappedField(row csvextract, fieldName string) {
	if strings.Contains(fieldName, "[N]") {
		// Check if we haven't already stored this pattern
		alreadyStored := false
		for _, stored := range dynamicFieldPatterns {
			if stored.FromMDOC == fieldName {
				alreadyStored = true
				break
			}
		}
		if !alreadyStored && strings.Contains(row.OSCEM, "[N]") {
			newRow := csvextract{
				OSCEM:          row.OSCEM,
				FromMDOC:       fieldName,
				OptionalsMDOC:  row.OptionalsMDOC,
				Units:          row.Units,
				CrunchFromMDOC: row.CrunchFromMDOC,
				Type:           row.Type,
			}
			dynamicFieldPatterns = append(dynamicFieldPatterns, newRow)
		}
	}
}

// Processes standard field mappings that don't involve arrays.
//
// Parameters:
//   - result: The output map being built
//   - row: CSV mapping rule for this field
//   - rawValues: Values found in the input data
//   - crunchFactor: Unit conversion factor to apply
func handleRegularField(result map[string]interface{}, row csvextract, rawValues []string, crunchFactor string) {
	if len(rawValues) > 0 {
		// Process the first value (apply unit conversion and type casting)
		value := processValue(rawValues[0], crunchFactor, row)
		// Insert the value at the specified path in the output structure
		insertNested(result, strings.Split(row.OSCEM, "."), value)
	}
}

// Processes fields that contain the [N] notation, creating arrays in the output structure.
// It parses the array path, ensures the array exists, and adds values to the appropriate array elements.
//
// Parameters:
//   - result: The output map being built
//   - row: CSV mapping rule for this array field
//   - rawValues: Values found in the input data
//   - crunchFactor: Unit conversion factor to apply
func handleArrayField(result map[string]interface{}, row csvextract, rawValues []string, crunchFactor string) {
	// Parse the array path (e.g., "acquisition.detectors[N].mode" -> ["acquisition"], "detectors", "mode")
	arrayPath, arrayName, propertyName := parseArrayPath(row.OSCEM)

	// Navigate to the parent container of the array
	parent := result
	for _, segment := range arrayPath {
		if _, exists := parent[segment]; !exists {
			parent[segment] = make(map[string]interface{})
		}
		parent = parent[segment].(map[string]interface{})
	}
	// Ensure the array exists
	if _, exists := parent[arrayName]; !exists {
		parent[arrayName] = make([]interface{}, 0)
	}
	// Add values to array elements
	arr := parent[arrayName].([]interface{})
	for i, rawValue := range rawValues {
		if rawValue == "" {
			continue // Skip empty values
		}
		// Process the value (apply unit conversion and type casting)
		value := processValue(rawValue, crunchFactor, row)
		// Ensure array has enough elements
		for len(arr) < i+1 {
			arr = append(arr, make(map[string]interface{}))
		}
		// Insert the value into the correct array element at the specified property path
		insertNested(arr[i].(map[string]interface{}), strings.Split(propertyName, "."), value)
	}
	parent[arrayName] = arr
}

// Parses an OSCEM path containing the [N] notation into its components.
// It separates the parent path, array name, and property name for array field processing.
// Example: "acquisition.detectors[N].mode" ->
//   - arrayParentPath: ["acquisition"]
//   - arrayName: "detectors"
//   - propertyName: "mode"
//
// Parameters:
//   - oscem: The OSCEM field path containing [N] notation
//
// Returns:
//   - []string: Parent path segments leading to the array
//   - string: Name of the array field
//   - string: Property name within each array element
func parseArrayPath(oscem string) ([]string, string, string) {
	// e.g., "acquisition.detectors[N].mode"
	parts := strings.Split(oscem, "[N]")
	beforeArray := parts[0] // "acquisition.detectors"
	afterArray := parts[1]  // ".mode"
	// Split the before part to get parent path and array name
	beforeParts := strings.Split(beforeArray, ".")
	arrayParentPath := beforeParts[:len(beforeParts)-1] // ["acquisition"]
	arrayName := beforeParts[len(beforeParts)-1]        // "detectors"
	propertyName := strings.TrimPrefix(afterArray, ".") // "mode"

	return arrayParentPath, arrayName, propertyName
}

// Applies unit conversion and type casting to a raw string value.
func processValue(rawValue, crunchFactor string, row csvextract) interface{} {
	// Apply unit conversion if a conversion factor is specified
	processedValue := applyUnitCrunch(crunchFactor, rawValue, row)
	// Cast to the appropriate data type based on the CSV mapping
	return castToBaseType(processedValue, row.Type, row.Units)
}

// Applies unit conversion to a raw value if a conversion factor is specified.
func applyUnitCrunch(crunchFactor string, rawValue string, row csvextract) string {
	// Apply unit conversion if crunch factor is defined
	if crunchFactor != "" {
		converted, err := unitCrunch(rawValue, crunchFactor)
		if err == nil {
			rawValue = converted
		} else {
			fmt.Fprintln(os.Stderr, "Unit crunching failed for", row.OSCEM, ":", err)
		}
	}
	return rawValue
}

// Applies a multiplication factor to a numeric string value for unit conversion.
func unitCrunch(value string, factor string) (string, error) {
	check, err := strconv.ParseFloat(value, 64)
	fac, _ := strconv.ParseFloat(factor, 64)
	if err != nil {
		return value, err
	}
	val := check * fac
	back := strconv.FormatFloat(val, 'f', 16, 64)

	return back, nil
}

// Converts a string value to the appropriate data type based on the type specification.
func castToBaseType(value string, t string, unit string) interface{} {
	switch strings.ToLower(t) {
	case "int":
		var val int64
		fmt.Sscanf(value, "%d", &val)
		var out basetypes.Int
		out.Set(val, unit) // sets .HasSet = true
		return out

	case "float", "float64":
		var val float64
		fmt.Sscanf(value, "%f", &val)
		var out basetypes.Float64
		out.Set(val, unit) // sets .HasSet = true
		return out

	case "bool":
		var out basetypes.Bool
		out.Set(strings.ToLower(value) == "true") // sets .HasSet = true
		return out

	case "string":
		var out basetypes.String
		out.Set(value) // sets .HasSet = true
		return out

	default:
		return nil
	}
}

// Inserts a value into a nested map structure at the specified path.
func insertNested(obj map[string]interface{}, path []string, val interface{}) {
	curr := obj
	for i, key := range path {
		if i == len(path)-1 {
			// Last key in path - set the value
			curr[key] = val
		} else {
			// Intermediate key - ensure nested map exists
			if _, ok := curr[key]; !ok {
				curr[key] = make(map[string]interface{})
			}
			curr = curr[key].(map[string]interface{})
		}
	}
}
