package conversion

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

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
