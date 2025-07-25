package conversion

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/osc-em/Converter/basetypes"
)

//go:embed conversions.csv
var embedded embed.FS

type FieldSpec struct {
	Path []string
	Type string
}

func Convert(jsonin []byte, contentFlag string, p1Flag string, p2Flag string, oFlag string) ([]byte, error) {
	var rows []csvextract
	if contentFlag != "" {
		var err error
		rows, err = loadMappingCSV(contentFlag) // custom
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
	} else {
		var err error
		rows, err = readCSVFile(embedded) // default
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
	}

	var values map[string]string
	_ = json.Unmarshal(jsonin, &values)

	out, err := convertToHierarchicalJSON(rows, values)
	if err != nil {
		log.Fatal(err)
	}
	// placeholder for adding from flags later
	cs := p1Flag
	gainref_flip_rotate := p2Flag

	casted := castToBaseType(cs, "float64", "mm")
	casted2 := castToBaseType(gainref_flip_rotate, "string", "")

	insertNested(out, []string{"instrument", "cs"}, casted)
	insertNested(out, []string{"acquisition", "gainref_flip_rotate"}, casted2)

	// this allows us to obtain nil values for types where Go usually doesnt allow them e.g. int
	cleaned := CleanMap(out)

	pretty, _ := json.MarshalIndent(cleaned, "", "  ")
	if oFlag == "" {
		cwd, _ := os.Getwd()
		cut := strings.Split(cwd, string(os.PathSeparator))
		name := cut[len(cut)-1] + ".json"
		os.WriteFile(name, pretty, 0644)
		fmt.Println()
		fmt.Println("Extracted data was written to: ", name)

	} else {
		twd := oFlag
		if !strings.Contains(twd, ".json") {
			var conc []string
			conc = append(conc, twd, "json")
			twd = strings.Join(conc, ".")
		}
		os.WriteFile(twd, pretty, 0644)
		fmt.Println()
		fmt.Printf("Extracted data was written to: %s", twd)
	}

	return pretty, nil
}

type csvextract struct {
	OSCEM          string
	FromXML        string
	FromMDOC       string
	OptionalsMDOC  string
	Units          string
	CrunchFromXML  string
	CrunchFromMDOC string
	OptionalsXML   string
	Type           string
}

func loadMappingCSV(mappingPath string) ([]csvextract, error) {
	// Use alternative file on disk (csvextractNew format)
	file, err := os.Open(mappingPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open mapping file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Normalize headers
	for i, h := range header {
		header[i] = strings.TrimLeft(strings.TrimSpace(h), "\ufeff")
	}

	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"oscem", "fromformat", "optionals", "units", "crunch", "type"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var rows []csvextract
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}

		newRow := csvextract{
			OSCEM:          row[colIdx["oscem"]],
			FromMDOC:       row[colIdx["fromformat"]],
			OptionalsMDOC:  row[colIdx["optionals"]],
			Units:          row[colIdx["units"]],
			CrunchFromMDOC: row[colIdx["crunch"]],
			Type:           row[colIdx["type"]],
		}
		rows = append(rows, newRow)
	}
	return rows, nil
}

// Read and parse the mapping CSV file
func readCSVFile(content embed.FS) ([]csvextract, error) {
	file, err := content.Open("conversions.csv")
	if err != nil {
		return nil, fmt.Errorf("could not open conversions.csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Normalize headers
	header := records[0]
	for i, h := range header {
		header[i] = strings.TrimLeft(h, "\ufeff")
	}

	// Map header names to column indices
	columnIndices := make(map[string]int)
	for idx, col := range header {
		columnIndices[strings.ToLower(strings.TrimSpace(col))] = idx
	}

	// List of required columns
	requiredCols := []string{
		"oscem", "fromxml", "frommdoc", "optionals_mdoc", "units",
		"crunchfromxml", "crunchfrommdoc", "optionals_xml", "type",
	}

	// Check all required columns exist
	for _, col := range requiredCols {
		if _, ok := columnIndices[col]; !ok {
			log.Fatalf("Required column %q not found in CSV header", col)
		}
	}

	var rows []csvextract

	for _, row := range records[1:] {
		data := csvextract{
			OSCEM:          row[columnIndices["oscem"]],
			FromXML:        row[columnIndices["fromxml"]],
			FromMDOC:       row[columnIndices["frommdoc"]],
			OptionalsMDOC:  row[columnIndices["optionals_mdoc"]],
			Units:          row[columnIndices["units"]],
			CrunchFromXML:  row[columnIndices["crunchfromxml"]],
			CrunchFromMDOC: row[columnIndices["crunchfrommdoc"]],
			OptionalsXML:   row[columnIndices["optionals_xml"]],
			Type:           row[columnIndices["type"]],
		}
		rows = append(rows, data)
	}

	return rows, nil
}

func CleanMap(data interface{}) interface{} {
	switch v := data.(type) {

	case map[string]interface{}:
		cleanedMap := make(map[string]interface{})
		for key, value := range v {
			cleanedValue := CleanMap(value)
			if cleanedValue != nil {
				cleanedMap[key] = cleanedValue
			}
		}
		if len(cleanedMap) == 0 {
			return nil
		}
		return cleanedMap

	case []interface{}:
		var cleanedSlice []interface{}
		for _, elem := range v {
			cleanedElem := CleanMap(elem)
			if cleanedElem != nil {
				cleanedSlice = append(cleanedSlice, cleanedElem)
			}
		}
		if len(cleanedSlice) == 0 {
			return nil
		}
		return cleanedSlice

	case basetypes.Int:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.Float64:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.Bool:
		if v.HasSet {
			return v
		}
		return nil
	case basetypes.String:
		if v.HasSet {
			return v
		}
		return nil

	default:
		// Primitive types that are set directly
		if v == nil {
			return nil
		}
		return v
	}
}
