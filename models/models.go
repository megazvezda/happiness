package models

import "time"

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
