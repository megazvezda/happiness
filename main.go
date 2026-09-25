package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coregx/gxpdf"
)

// RFC 5545: "YYYYMMDDTHHMMSS"
const icsDateFormat = "20060102T150405"

// I think it's worth to briefly explain what does these regexp patterns do, as this is just pure magic to me,
// thankfully there are LLM's to write them for me.

// lecturePattern matches a table row like
// "3 12:10-13:45 2 0 Meow Concepts (ELESB16513) P2 118 Prof Dr. Magnus Manavičius Lectures".
// Groups 1-5 capture position, start/end time, week number (1/2, or empty for every week), and subgroup.
// Groups 6-8 (subject, auditorium, lecturer) are free text with no fixed shape, so the lazy group 8 (+?) yields to group 9's
// fixed keyword list (Lectures/Practical exercises/Laboratory work), which anchors where the line actually ends.
var lecturePattern = regexp.MustCompile(`(?m)^(\d+)\s+(\d{2}:\d{2})-(\d{2}:\d{2})([0-2]?)\s+(\d+)\s+([^\r\n]+)\s+([^\r\n]+)\s+([^\r\n]+?)\s*(Lectures|Practical exercises[^\r\n]*|Laboratory work[^\r\n]*)`)

// weekdayPattern matches a day heading like "Monday 2026-09-01 — 2026-12-20",
// capturing the weekday name and the semester start/end dates.
// [—-] accepts either an em dash or a hyphen since PDF extraction sometimes flattens the "—".
// It's run with FindAllStringSubmatchIndex so parseSchedule gets byte offsets to slice each day's section out of the text.
var weekdayPattern = regexp.MustCompile(`(?m)(Monday|Tuesday|Wednesday|Thursday|Friday)\s+(\d{4}-\d{2}-\d{2})\s+[—-]\s+(\d{4}-\d{2}-\d{2})`)

// Lecture in this context could also mean a practical lecture or a laboratory work.
type Lecture struct {
	Position    uint8
	StartTime   time.Time
	EndTime     time.Time
	Week        uint8 // enum todo
	Subgroup    uint8 // same todo
	SubjectName string
	Auditorium  string
	Lecturer    string
	Type        string
}

type Day []Lecture

type Week []Day

func main() {
	inputPath := flag.String("input", "timetable_2026-09-12.pdf", "timetable PDF path")
	outputPath := flag.String("output", "timetable.ics", "calendar output path")
	flag.Parse()

	doc, err := gxpdf.Open(*inputPath)
	if err != nil {
		log.Fatalf("failed to open PDF: %v", err)
	}
	defer doc.Close()

	schedule, err := parseSchedule(doc.ExtractTables())
	if err != nil {
		log.Fatal(err)
	}
	if err := writeICS(*outputPath, schedule); err != nil {
		log.Fatalf("failed to write calendar: %v", err)
	}
	log.Printf("wrote %s", *outputPath)
}

type Schedule struct {
	Week      Week
	StartDate time.Time
	EndDate   time.Time
}

func parseSchedule(tables []*gxpdf.Table) (Schedule, error) {
	var rows []string
	for _, table := range tables {
		for _, row := range table.Rows() {
			for _, text := range row {
				text = strings.ReplaceAll(text, "\r\n", " ")
				text = strings.ReplaceAll(text, "\n", " ")
				rows = append(rows, text)
			}
		}
	}
	return parseScheduleText(strings.Join(rows, "\n"))
}

func parseScheduleText(text string) (Schedule, error) {
	week := make(Week, 5)
	for i := range week {
		week[i] = Day{}
	}
	dayIndexes := map[string]int{
		"Monday": 0, "Tuesday": 1, "Wednesday": 2, "Thursday": 3, "Friday": 4,
	}
	var startDate, endDate time.Time

	headings := weekdayPattern.FindAllStringSubmatchIndex(text, -1)
	for headingIndex, heading := range headings {
		dayIndex, ok := dayIndexes[text[heading[2]:heading[3]]]
		if !ok {
			continue
		}
		headingStart, err := time.ParseInLocation("2006-01-02", text[heading[4]:heading[5]], time.Local)
		if err != nil {
			return Schedule{}, fmt.Errorf("invalid semester start date: %w", err)
		}
		headingEnd, err := time.ParseInLocation("2006-01-02", text[heading[6]:heading[7]], time.Local)
		if err != nil {
			return Schedule{}, fmt.Errorf("invalid semester end date: %w", err)
		}
		if startDate.IsZero() || headingStart.Before(startDate) {
			startDate = headingStart
		}
		if endDate.IsZero() || headingEnd.After(endDate) {
			endDate = headingEnd.Add(24*time.Hour - time.Second)
		}
		sectionEnd := len(text)
		if headingIndex+1 < len(headings) {
			sectionEnd = headings[headingIndex+1][0]
		}
		for _, match := range lecturePattern.FindAllStringSubmatch(text[heading[1]:sectionEnd], -1) {
			lecture, err := parseLecture(match)
			if err != nil {
				return Schedule{}, err
			}
			week[dayIndex] = append(week[dayIndex], lecture)
		}
	}
	return Schedule{
		Week:      week,
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

func parseLecture(match []string) (Lecture, error) {
	if len(match) != 10 {
		return Lecture{}, fmt.Errorf("unexpected lecture match with %d fields", len(match))
	}

	position, err := strconv.ParseUint(match[1], 10, 8)
	if err != nil {
		return Lecture{}, fmt.Errorf("invalid lecture position %q: %w", match[1], err)
	}
	startTime, err := time.Parse("15:04", match[2])
	if err != nil {
		return Lecture{}, fmt.Errorf("invalid start time %q: %w", match[2], err)
	}
	endTime, err := time.Parse("15:04", match[3])
	if err != nil {
		return Lecture{}, fmt.Errorf("invalid end time %q: %w", match[3], err)
	}

	weekNumber := uint64(0)
	if match[4] != "" {
		weekNumber, err = strconv.ParseUint(match[4], 10, 8)
		if err != nil || weekNumber > 2 {
			return Lecture{}, fmt.Errorf("invalid week number %q", match[4])
		}
	}
	subgroup, err := strconv.ParseUint(match[5], 10, 8)
	if err != nil {
		return Lecture{}, fmt.Errorf("invalid subgroup %q: %w", match[5], err)
	}

	return Lecture{
		Position:    uint8(position),
		StartTime:   startTime,
		EndTime:     endTime,
		Week:        uint8(weekNumber),
		Subgroup:    uint8(subgroup),
		SubjectName: strings.TrimSpace(match[6]),
		Auditorium:  strings.TrimSpace(match[7]),
		Lecturer:    strings.TrimSpace(match[8]),
		Type:        strings.TrimSpace(match[9]),
	}, nil
}

func writeICS(path string, schedule Schedule) error {
	// this is implemented after it was decided to start the lecture times from their provided date range (e.g: "WeekdayName 2026-09-01 — 2026-12-20")
	if schedule.StartDate.IsZero() {
		return fmt.Errorf("schedule has no semester start date")
	}
	if schedule.EndDate.IsZero() {
		return fmt.Errorf("schedule has no semester end date")
	}
	monday := startOfWeek(schedule.StartDate)

	var output strings.Builder
	output.WriteString("BEGIN:VCALENDAR\r\n")
	output.WriteString("VERSION:2.0\r\n")
	output.WriteString("PRODID:-//megazvezda/happiness//Timetable//EN\r\n")
	output.WriteString("CALSCALE:GREGORIAN\r\n")

	for dayIndex, day := range schedule.Week {
		if dayIndex >= 7 {
			return fmt.Errorf("timetable contains more than seven weekday tables")
		}
		for lectureIndex, lecture := range day {
			if lecture.Week > 2 {
				return fmt.Errorf("lecture %d on weekday %d has invalid week %d", lectureIndex, dayIndex, lecture.Week)
			}
			start := monday.AddDate(0, 0, dayIndex)
			if start.Before(schedule.StartDate) {
				start = start.AddDate(0, 0, 7)
			}
			if lecture.Week == 2 {
				start = start.AddDate(0, 0, 7)
			}
			start = time.Date(start.Year(), start.Month(), start.Day(), lecture.StartTime.Hour(), lecture.StartTime.Minute(), 0, 0, time.UTC)
			end := time.Date(start.Year(), start.Month(), start.Day(), lecture.EndTime.Hour(), lecture.EndTime.Minute(), 0, 0, time.UTC)

			output.WriteString("BEGIN:VEVENT\r\n")
			output.WriteString("UID:" + eventUID(dayIndex, lecture) + "\r\n")
			// "Z\r\n" is used here to specify that DTSTAMP is UTC in this case
			output.WriteString("DTSTAMP:" + time.Now().UTC().Format(icsDateFormat) + "Z\r\n")
			output.WriteString("DTSTART:" + start.Format(icsDateFormat) + "\r\n")
			output.WriteString("DTEND:" + end.Format(icsDateFormat) + "\r\n")
			if lecture.Week == 0 {
				output.WriteString("RRULE:FREQ=WEEKLY;UNTIL=" + schedule.EndDate.Format(icsDateFormat) + "\r\n")
			} else {
				output.WriteString("RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=" + schedule.EndDate.Format(icsDateFormat) + "\r\n")
			}
			output.WriteString("SUMMARY:" + escapeICS(lecture.SubjectName) + "\r\n")
			output.WriteString("LOCATION:" + escapeICS(lecture.Auditorium) + "\r\n")
			output.WriteString("DESCRIPTION:" + escapeICS(fmt.Sprintf("%s; subgroup %d", lecture.Type, lecture.Subgroup)) + "\r\n")
			output.WriteString("END:VEVENT\r\n")
		}
	}
	output.WriteString("END:VCALENDAR\r\n")

	return os.WriteFile(path, []byte(output.String()), 0644)
}

func startOfWeek(date time.Time) time.Time {
	daysSinceMonday := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -daysSinceMonday)
}

func eventUID(dayIndex int, lecture Lecture) string {
	value := fmt.Sprintf("%d|%d|%s|%s|%s|%s|%d", dayIndex, lecture.Position, lecture.SubjectName, lecture.Auditorium, lecture.StartTime.Format("15:04"), lecture.EndTime.Format("15:04"), lecture.Week)
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:]) + "@happiness"
}

func escapeICS(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, ";", `\;`)
	value = strings.ReplaceAll(value, ",", `\,`)
	value = strings.ReplaceAll(value, "\r\n", `\n`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}
