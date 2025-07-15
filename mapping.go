package conversion

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func convertToHierarchicalJSON(rows []csvextract, input map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Process regular mappings first
	processRegularMappings(result, rows, input)

	// Then process dynamic detectors
	processDynamicDetectors(result, input, rows)

	return result, nil
}

func processRegularMappings(result map[string]interface{}, rows []csvextract, input map[string]string) {
	for _, row := range rows {
		rawValues, crunchFactor, found := findMatchingValues(row, input, extractValuesFromInput)
		if !found {
			continue
		}

		if strings.Contains(row.OSCEM, "[N]") {
			handleArrayField(result, row, rawValues, crunchFactor)
		} else {
			handleRegularField(result, row, rawValues, crunchFactor)
		}
	}
}

// Define a type for the extraction function
type ValueExtractor func(map[string]string, string) ([]string, bool)

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
			if values, found := extractor(input, check.field); found {
				return values, check.crunch, true
			}
		}
	}
	return nil, "", false
}

func handleArrayField(result map[string]interface{}, row csvextract, rawValues []string, crunchFactor string) {
	arrayPath, arrayName, propertyName := parseArrayPath(row.OSCEM)

	// Navigate to parent
	parent := result
	for _, segment := range arrayPath {
		if _, exists := parent[segment]; !exists {
			parent[segment] = make(map[string]interface{})
		}
		parent = parent[segment].(map[string]interface{})
	}
	// Ensure array exists
	if _, exists := parent[arrayName]; !exists {
		parent[arrayName] = make([]interface{}, 0)
	}

	// Add values to array
	arr := parent[arrayName].([]interface{})
	for i, rawValue := range rawValues {
		if rawValue == "" {
			continue
		}

		value := processValue(rawValue, crunchFactor, row)
		// Ensure array has enough size
		for len(arr) < i+1 {
			arr = append(arr, make(map[string]interface{}))
		}
		setArrayProperty(arr, i, propertyName, value)
	}
	parent[arrayName] = arr
}

func handleRegularField(result map[string]interface{}, row csvextract, rawValues []string, crunchFactor string) {
	if len(rawValues) > 0 {
		value := processValue(rawValues[0], crunchFactor, row)
		insertNested(result, strings.Split(row.OSCEM, "."), value)
	}
}

func processValue(rawValue, crunchFactor string, row csvextract) interface{} {
	processedValue := applyUnitCrunch(crunchFactor, rawValue, row)
	return castToBaseType(processedValue, row.Type, row.Units)
}

func extractValuesFromInput(input map[string]string, key string) ([]string, bool) {
	if strings.Contains(key, ";") {
		// Handle semicolon-separated field names
		fieldNames := strings.Split(key, ";")
		var result []string
		var foundAny bool

		for _, fieldName := range fieldNames {
			fieldName = strings.TrimSpace(fieldName)
			if fieldName == "" {
				result = append(result, "") // Keep empty for alignment
				continue
			}

			if val, exists := input[fieldName]; exists {
				result = append(result, val)
				foundAny = true
			} else {
				result = append(result, "") // Keep empty for alignment
			}
		}
		return result, foundAny
	} else {
		// Handle single field name lookup
		if val, exists := input[key]; exists {
			return []string{val}, true
		}
		return nil, false
	}
}

func parseArrayPath(oscem string) ([]string, string, string) {
	// example: "acquisition.detectors[N].mode"
	parts := strings.Split(oscem, "[N]")
	beforeArray := parts[0] // "acquisition.detectors"
	afterArray := parts[1]  // ".mode"

	beforeParts := strings.Split(beforeArray, ".")
	arrayParentPath := beforeParts[:len(beforeParts)-1] // ["acquisition"]
	arrayName := beforeParts[len(beforeParts)-1]        // "detectors"
	propertyName := strings.TrimPrefix(afterArray, ".") // "mode"

	return arrayParentPath, arrayName, propertyName
}

func setArrayProperty(arr []interface{}, index int, propertyName string, value interface{}) {
	if strings.Contains(propertyName, ".") {
		insertNested(arr[index].(map[string]interface{}), strings.Split(propertyName, "."), value)
	} else {
		arr[index].(map[string]interface{})[propertyName] = value
	}
}

func insertNested(obj map[string]interface{}, path []string, val interface{}) {
	curr := obj
	for i, key := range path {
		if i == len(path)-1 {
			curr[key] = val
		} else {
			if _, ok := curr[key]; !ok {
				curr[key] = make(map[string]interface{})
			}
			curr = curr[key].(map[string]interface{})
		}
	}
}

func unitCrunch(v string, fac string) (string, error) {
	check, err := strconv.ParseFloat(v, 64)
	factor, _ := strconv.ParseFloat(fac, 64)
	if err != nil {
		return v, err
	}
	val := check * factor
	back := strconv.FormatFloat(val, 'f', 16, 64)

	return back, nil
}

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
