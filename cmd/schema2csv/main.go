package main

import (
	"fmt"
	"os"

	"github.com/osc-em/Converter/pkg/schema2csv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: schema2csv <schema-url>")
		os.Exit(1)
	}
	schemaURL := os.Args[1]

	if err := schema2csv.Run(schemaURL, "schema_template.csv"); err != nil {
		fmt.Println("❌ Error:", err)
		os.Exit(1)
	}
	fmt.Println("✅ CSV created: schema_template.csv")
}
