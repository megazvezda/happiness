package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseScheduleTextIncludesContinuationRows(t *testing.T) {
	schedule, err := parseScheduleText(`Friday 2026-09-01 — 2026-12-20
1
08:30-10:05
0
Digital Devices
P2 158
Lecturer
Laboratory work
2
10:20-11:55
0
Digital Devices
P2 203
Lecturer
Laboratory work`)
	if err != nil {
		t.Fatalf("parseScheduleText returned error: %v", err)
	}
	if got := len(schedule.Week[4]); got != 2 {
		t.Fatalf("got %d Friday lectures, want 2", got)
	}
	if got := schedule.Week[4][1].Auditorium; got != "P2 203" {
		t.Fatalf("got continuation auditorium %q, want %q", got, "P2 203")
	}
}

func TestWriteICSUsesFirstWeekdayOnOrAfterSemesterStart(t *testing.T) {
	schedule := Schedule{
		Week: Week{
			{{
				StartTime:   time.Date(0, 1, 1, 10, 20, 0, 0, time.Local),
				EndTime:     time.Date(0, 1, 1, 11, 55, 0, 0, time.Local),
				SubjectName: "Monday class",
			}},
			{{
				StartTime:   time.Date(0, 1, 1, 12, 10, 0, 0, time.Local),
				EndTime:     time.Date(0, 1, 1, 13, 45, 0, 0, time.Local),
				SubjectName: "Tuesday class",
			}},
			{},
			{},
			{{
				StartTime:   time.Date(0, 1, 1, 14, 30, 0, 0, time.Local),
				EndTime:     time.Date(0, 1, 1, 16, 5, 0, 0, time.Local),
				SubjectName: "Friday class",
				Week:        2,
			}},
		},
		StartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local),
		EndDate:   time.Date(2026, 12, 20, 23, 59, 59, 0, time.Local),
	}

	data, err := writeICS(schedule)
	if err != nil {
		t.Fatalf("writeICS returned error: %v", err)
	}
	output := string(data)
	for _, expected := range []string{
		"DTSTART:20260907T102000\r\n",
		"DTSTART:20260901T121000\r\n",
		"DTSTART:20260911T143000\r\n",
		"RRULE:FREQ=WEEKLY;UNTIL=20261220T235959\r\n",
		"RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=20261220T235959\r\n",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("generated ICS does not contain %q", expected)
		}
	}
	if strings.Contains(output, "UNTIL=20261220T235959Z") {
		t.Error("generated ICS uses a UTC UNTIL with floating event times")
	}
}
