//go:build !js

package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	inputPath := flag.String("input", "timetable_2026-09-12.pdf", "timetable PDF path")
	outputPath := flag.String("output", "timetable.ics", "calendar output path")
	flag.Parse()

	doc, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("failed to open PDF: %v", err)
	}

	schedule, err := doConversion(doc)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*outputPath, schedule, 0644); err != nil {
		log.Fatalf("failed to write calendar: %v", err)
	}
	log.Printf("wrote %s", *outputPath)
}
