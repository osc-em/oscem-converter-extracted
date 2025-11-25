package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	conversion "github.com/osc-em/oscem-converter-extracted"
)

func main() {
	inputFile := flag.String("i", "", "Input JSON file (required)")
	outputFile := flag.String("o", "", "Output JSON file name (optional)")
	mappingFile := flag.String("map", "", "Custom CSV mapping file path (optional)")
	p1Flag := flag.String("cs", "", "Provide CS (spherical aberration) value here (optional)")
	p2Flag := flag.String("gain_flip_rotate", "", "Provide whether and how to flip the gain ref here, if applicaple (optional)")

	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Input file (-in) is required.")
	}

	jsonIn, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}
	_, err1 := conversion.Convert(jsonIn, *mappingFile, *p1Flag, *p2Flag, *outputFile)
	if err1 != nil {
		fmt.Fprintln(os.Stderr, "conversion failed because", err)
	}
}
