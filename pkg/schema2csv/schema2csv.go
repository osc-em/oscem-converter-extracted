package schema2csv

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var typeMap = map[string]string{
	"string":  "String",
	"integer": "Int",
	"number":  "Float64",
	"boolean": "Bool",
}

// Run executes the full pipeline: fetch schema, flatten, write CSV
func Run(url, outputCSV string) error {
	schema, err := FetchSchema(url)
	if err != nil {
		return err
	}

	rows := FlattenSchema(schema)

	if err := WriteCSV(rows, outputCSV); err != nil {
		return err
	}
	return nil
}

// FetchSchema fetches a JSON schema from a raw Git URL
func FetchSchema(url string) (map[string]interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch schema: %s\n%s", resp.Status, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// FlattenSchema flattens $defs and properties recursively
func FlattenSchema(schema map[string]interface{}) [][2]string {
	rows := [][2]string{}

	defsRaw, ok := schema["$defs"].(map[string]interface{})
	if !ok {
		fmt.Println("No $defs found in schema")
		return rows
	}

	for defName, defValue := range defsRaw {
		if defMap, ok := defValue.(map[string]interface{}); ok {
			rows = append(rows, flattenDef(defName, defMap, defsRaw, "")...)
		}
	}
	return rows
}

// flattenDef flattens a single definition recursively
func flattenDef(prefix string, def map[string]interface{}, defs map[string]interface{}, parentPath string) [][2]string {
	rows := [][2]string{}

	propsRaw, ok := def["properties"].(map[string]interface{})
	if !ok {
		return rows
	}

	for propName, propVal := range propsRaw {
		propMap, ok := propVal.(map[string]interface{})
		if !ok {
			continue
		}

		path := propName
		if parentPath != "" {
			path = parentPath + "." + propName
		} else {
			path = prefix + "." + propName
		}

		rows = append(rows, flattenProperty(path, propMap, defs)...)
	}
	return rows
}

// flattenProperty handles primitives, $ref, anyOf, arrays recursively
func flattenProperty(path string, prop map[string]interface{}, defs map[string]interface{}) [][2]string {
	rows := [][2]string{}

	// Handle $ref
	if ref, ok := prop["$ref"].(string); ok {
		refName := strings.TrimPrefix(ref, "#/$defs/")
		if def, found := defs[refName].(map[string]interface{}); found {
			rows = append(rows, flattenDef(refName, def, defs, path)...)
		}
		return rows
	}

	// Handle anyOf (nullable or union types)
	if anyOf, ok := prop["anyOf"].([]interface{}); ok {
		for _, item := range anyOf {
			if itemMap, ok := item.(map[string]interface{}); ok {
				rows = append(rows, flattenProperty(path, itemMap, defs)...)
			}
		}
		return rows
	}

	// Handle arrays and primitive types
	if t, ok := prop["type"]; ok {
		switch tt := t.(type) {
		case string:
			if tt == "array" {
				if items, ok := prop["items"].(map[string]interface{}); ok {
					rows = append(rows, flattenProperty(path+"[N]", items, defs)...)
				}
			} else if mapped, exists := typeMap[tt]; exists && mapped != "" {
				rows = append(rows, [2]string{path, mapped})
			}
		case []interface{}:
			for _, tItem := range tt {
				if tStr, ok := tItem.(string); ok {
					if tStr == "array" {
						if items, ok := prop["items"].(map[string]interface{}); ok {
							rows = append(rows, flattenProperty(path+"[N]", items, defs)...)
						}
					} else if mapped, exists := typeMap[tStr]; exists && mapped != "" {
						rows = append(rows, [2]string{path, mapped})
					}
				}
			}
		}
	}
	return rows
}

// WriteCSV writes flattened rows to a CSV file
func WriteCSV(rows [][2]string, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"oscem", "type"}); err != nil {
		return err
	}

	for _, row := range rows {
		if err := w.Write([]string{row[0], row[1]}); err != nil {
			return err
		}
	}
	return nil
}
