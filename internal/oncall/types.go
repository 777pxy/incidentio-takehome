// Package oncall holds the on-call schedule types and the parsing and
// validation of their JSON input files.
package oncall

import "time"

// Schedule is a rotation: users take turns being on call, handing over
// every HandoverIntervalDays days starting from HandoverStartAt.
type Schedule struct {
	Users                []string  `json:"users"`
	HandoverStartAt      time.Time `json:"handover_start_at"`
	HandoverIntervalDays int       `json:"handover_interval_days"`
}

// Override temporarily replaces whoever is on call between StartAt and EndAt.
type Override struct {
	User    string    `json:"user"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

// Entry is one block of the final rendered schedule.
type Entry struct {
	User    string    `json:"user"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}
