package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coregx/gxpdf"
	"github.com/megazvezda/happiness/models"
)

func main() {
	doc, err := gxpdf.Open("timetable_2026-09-12.pdf")
	if err != nil {
		log.Fatalf("failed to open the pdf, aborting\n")
	}
	defer doc.Close()

	tables := doc.ExtractTables()

	d1 := []string{}

	for _, table := range tables {
		rows := table.Rows()
		for _, row := range rows {
			for _, text := range row {
				d1 = append(d1, text)
			}
			//d1 = append(d1, "\n")
		}
		//d1 = append(d1, " ")
	}

	sanitizedRows := make([]string, len(d1))
	for i, row := range d1 {
		// handling both Unix \n and Windows \r\n
		cleaned := strings.ReplaceAll(row, "\r\n", " ")
		cleaned = strings.ReplaceAll(cleaned, "\n", " ")
		sanitizedRows[i] = cleaned
	}

	//data := string(strings.Join(sanitizedRows, "\n"))
	pattern := `(?m)^(\d+)\s+(\d{2}:\d{2})-(\d{2}:\d{2})([0-2]?)\s+(\d+)\s+([^\r\n]+)\s+([^\r\n]+)\s+([^\r\n]+?)\s*(Lectures|Practical exercises[^\r\n]*|Laboratory work[^\r\n]*)`
	re := regexp.MustCompile(pattern)

	data := strings.Join(sanitizedRows, "\n")
	matches := re.FindAllStringSubmatch(data, -1)

	d := models.Day{}
	for _, match := range matches {
		if len(match) == 10 {
			position, err := strconv.ParseUint(match[1], 10, 8)
			if err != nil {
				log.Fatalf("failed to convert to uint8")
			}
			startTime, err := time.Parse("15:04", match[2])
			if err != nil {
				log.Fatal("failed to convert startTime")
			}
			endTime, err := time.Parse("15:04", match[3])
			if err != nil {
				log.Fatalf("failed to convert endTime")
			}

			weekStr := match[4]
			if weekStr == "" {
				weekStr = "0"
			}
			week, err := strconv.ParseUint(weekStr, 10, 8)
			if err != nil {
				log.Fatalf("failed to convert to uint8")
			}
			subgroup, err := strconv.ParseUint(match[5], 10, 8)
			if err != nil {
				log.Fatalf("failed to convert to uint8")
			}

			l := models.Lecture{
				Position:    uint8(position),
				StartTime:   startTime,
				EndTime:     endTime,
				Week:        uint8(week),
				Subgroup:    uint8(subgroup),
				SubjectName: match[6],
				Auditorium:  match[7],
				Lecturer:    match[8],
				Type:        match[9],
			}
			d = append(d, l)
		}
	}
	w := models.Week{d}
	for _, day := range w {
		for _, lecture := range day {
			fmt.Printf("%v\n%v\n%v\n%v\n%v\n%v\n%v\n%v\n%v", lecture.Auditorium, lecture.StartTime, lecture.EndTime, lecture.Week, lecture.Subgroup, lecture.SubjectName, lecture.Auditorium, lecture.Lecturer, lecture.Type)
		}
		fmt.Print('\n')
	}
}
