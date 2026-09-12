package main

import (
	"fmt"
	"log"

	"github.com/coregx/gxpdf"
)

func main() {
	doc, err := gxpdf.Open("timetable_2026-09-12.pdf")
	if err != nil {
		log.Fatalf("failed to open the pdf, aborting\n")
	} else {
		fmt.Print("ok")
	}
	defer doc.Close()

	tables := doc.ExtractTables()
	for _, table := range tables {
		rows := table.Rows()
		for _, row := range rows {
			fmt.Println(row)
			fmt.Println("ok stop")
		}
	}

}
