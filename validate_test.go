// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRFC8984Corpus parses every RFC 8984 Section 6 example figure and
// asserts that it validates cleanly. The corpus is the curated set of valid
// objects: each is a conformant Event, Task, or Group from the spec, so a
// non-nil error from Validate would mean the validation pass is rejecting
// conformant input — including the floating-time event (§6.7) and the
// open-ended task with neither due nor start (§6.2), the two cases most likely
// to be over-constrained.
func TestValidateRFC8984Corpus(t *testing.T) {
	t.Parallel()

	matches, err := filepath.Glob(filepath.Join("testdata", "rfc8984", "*.json"))
	if err != nil {
		t.Fatalf("glob corpus: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no corpus files found under testdata/rfc8984")
	}

	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			obj, err := Parse(b)
			if err != nil {
				t.Fatalf("Parse(%s): %v", path, err)
			}

			v, ok := obj.(interface{ Validate() error })
			if !ok {
				t.Fatalf("parsed %T does not implement Validate", obj)
			}
			if err := v.Validate(); err != nil {
				t.Errorf("Validate(%s) = %v, want nil", path, err)
			}
		})
	}
}

// TestValidateRejects covers the curated invalid set: each case is a
// hand-built object that violates one MUST, and the test asserts that Validate
// rejects it with a *ValidationError naming the right property. Building the
// objects in Go (rather than from JSON) keeps each case pinned to exactly one
// violation, so the asserted property path is unambiguous.
func TestValidateRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func() error
		wantProp string
	}{
		{
			name: "event missing uid",
			validate: func() error {
				return (&Event{Type: typeEvent}).Validate()
			},
			wantProp: "uid",
		},
		{
			name: "event wrong @type",
			validate: func() error {
				return (&Event{Type: "Task", UID: "x"}).Validate()
			},
			wantProp: "@type",
		},
		{
			name: "event with due smuggled in via Extra",
			validate: func() error {
				return (&Event{
					Type:  typeEvent,
					UID:   "x",
					Extra: map[string]json.RawMessage{"due": json.RawMessage(`"2020-01-01T00:00:00"`)},
				}).Validate()
			},
			wantProp: "due",
		},
		{
			name: "task missing uid",
			validate: func() error {
				return (&Task{Type: typeTask}).Validate()
			},
			wantProp: "uid",
		},
		{
			name: "task wrong @type",
			validate: func() error {
				return (&Task{Type: "Event", UID: "x"}).Validate()
			},
			wantProp: "@type",
		},
		{
			name: "group missing uid",
			validate: func() error {
				return (&Group{Type: typeGroup}).Validate()
			},
			wantProp: "uid",
		},
		{
			name: "group wrong @type",
			validate: func() error {
				return (&Group{Type: "Event", UID: "x"}).Validate()
			},
			wantProp: "@type",
		},
		{
			name: "group entry with unknown @type",
			validate: func() error {
				return (&Group{
					Type: typeGroup,
					UID:  "x",
					Entries: []json.RawMessage{
						json.RawMessage(`{"@type":"Widget","uid":"y"}`),
					},
				}).Validate()
			},
			wantProp: "entries[0].@type",
		},
		{
			name: "group entry that is a nested Group",
			validate: func() error {
				return (&Group{
					Type: typeGroup,
					UID:  "x",
					Entries: []json.RawMessage{
						json.RawMessage(`{"@type":"Group","uid":"y"}`),
					},
				}).Validate()
			},
			wantProp: "entries[0].@type",
		},
		{
			// A valid Event precedes the bad entry, so the violation is
			// reported at index 1 — exercising the per-entry property path.
			name: "group entry missing @type at index 1",
			validate: func() error {
				return (&Group{
					Type: typeGroup,
					UID:  "x",
					Entries: []json.RawMessage{
						json.RawMessage(`{"@type":"Event","uid":"ok"}`),
						json.RawMessage(`{"uid":"y"}`),
					},
				}).Validate()
			},
			wantProp: "entries[1].@type",
		},
		{
			name: "recurrenceOverrides key is not a LocalDateTime",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceOverrides: map[string]PatchObject{
						"not-a-datetime": {"title": json.RawMessage(`"moved"`)},
					},
				}).Validate()
			},
			wantProp: "recurrenceOverrides/not-a-datetime",
		},
		{
			name: "recurrenceOverrides patch pointer targets root @type",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceOverrides: map[string]PatchObject{
						"2020-01-08T14:00:00": {"@type": json.RawMessage(`"Task"`)},
					},
				}).Validate()
			},
			wantProp: "recurrenceOverrides/2020-01-08T14:00:00/@type",
		},
		{
			name: "recurrenceOverrides patch pointer targets root uid",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceOverrides: map[string]PatchObject{
						"2020-01-08T14:00:00": {"uid": json.RawMessage(`"y"`)},
					},
				}).Validate()
			},
			wantProp: "recurrenceOverrides/2020-01-08T14:00:00/uid",
		},
		{
			name: "localizations patch pointer is malformed (bad escape)",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					Localizations: map[string]PatchObject{
						"fr": {"title~2": json.RawMessage(`"bonjour"`)},
					},
				}).Validate()
			},
			wantProp: "localizations/fr/title~2",
		},
		{
			name: "localizations patch pointer targets root @type",
			validate: func() error {
				return (&Task{
					Type: typeTask, UID: "x",
					Localizations: map[string]PatchObject{
						"de": {"@type": json.RawMessage(`"Event"`)},
					},
				}).Validate()
			},
			wantProp: "localizations/de/@type",
		},
		{
			name: "custom timeZone not present in timeZones",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					TimeZone: TimeZoneId("/custom"),
				}).Validate()
			},
			wantProp: "timeZone",
		},
		{
			name: "location custom timeZone not present in timeZones",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					Locations: map[Id]Location{
						"loc1": {TimeZone: TimeZoneId("/elsewhere")},
					},
				}).Validate()
			},
			wantProp: "locations/loc1/timeZone",
		},
		{
			name: "recurrence rule missing frequency",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceRules: []RecurrenceRule{{}},
				}).Validate()
			},
			wantProp: "recurrenceRules[0].frequency",
		},
		{
			name: "recurrence rule with unknown frequency",
			validate: func() error {
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceRules: []RecurrenceRule{{Frequency: "fortnightly"}},
				}).Validate()
			},
			wantProp: "recurrenceRules[0].frequency",
		},
		{
			name: "recurrence rule with both count and until",
			validate: func() error {
				count := uint(5)
				until := LocalDateTime{Year: 2020, Month: 12, Day: 31}
				return (&Event{
					Type: typeEvent, UID: "x",
					RecurrenceRules: []RecurrenceRule{{
						Frequency: FrequencyDaily,
						Count:     &count,
						Until:     &until,
					}},
				}).Validate()
			},
			wantProp: "recurrenceRules[0]",
		},
		{
			name: "excluded recurrence rule missing frequency",
			validate: func() error {
				return (&Task{
					Type: typeTask, UID: "x",
					ExcludedRecurrenceRules: []RecurrenceRule{{}},
				}).Validate()
			},
			wantProp: "excludedRecurrenceRules[0].frequency",
		},
		{
			name: "embedded timeZone transition rule with both count and until",
			validate: func() error {
				count := uint(2)
				until := LocalDateTime{Year: 2020, Month: 12, Day: 31}
				return (&Event{
					Type: typeEvent, UID: "x",
					TimeZones: map[TimeZoneId]TimeZone{
						"/custom": {Standard: []TimeZoneRule{{
							RecurrenceRules: []RecurrenceRule{{
								Frequency: FrequencyYearly,
								Count:     &count,
								Until:     &until,
							}},
						}}},
					},
				}).Validate()
			},
			wantProp: "timeZones//custom/standard[0].recurrenceRules[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want a *ValidationError naming %q", tt.wantProp)
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() = %v (%T), want a *ValidationError", err, err)
			}
			if verr.Property != tt.wantProp {
				t.Errorf("ValidationError.Property = %q, want %q (err: %v)",
					verr.Property, tt.wantProp, err)
			}
		})
	}
}

// TestValidateEmptyTypeAccepted asserts the codec's not-yet-stamped form is
// accepted: an object whose Type field is the zero value validates as the
// implied correct type, because MarshalJSON forces "@type" onto the wire and a
// zero Type is therefore the un-stamped form of the right type, not a wrong
// one.
func TestValidateEmptyTypeAccepted(t *testing.T) {
	t.Parallel()

	if err := (&Event{UID: "x"}).Validate(); err != nil {
		t.Errorf("Event{UID} with empty Type: Validate() = %v, want nil", err)
	}
	if err := (&Task{UID: "x"}).Validate(); err != nil {
		t.Errorf("Task{UID} with empty Type: Validate() = %v, want nil", err)
	}
	if err := (&Group{UID: "x"}).Validate(); err != nil {
		t.Errorf("Group{UID} with empty Type: Validate() = %v, want nil", err)
	}
}

// TestValidateFloatingStartValid asserts that a start with no timeZone — the
// floating-time case (Section 1.4.4) — is accepted for both Event and Task. The
// absence of a zone is the spec's floating signal, so it must not be flagged.
func TestValidateFloatingStartValid(t *testing.T) {
	t.Parallel()

	start := LocalDateTime{}
	if err := (&Event{Type: typeEvent, UID: "x", Start: &start}).Validate(); err != nil {
		t.Errorf("Event with floating start: Validate() = %v, want nil", err)
	}
	if err := (&Task{Type: typeTask, UID: "x", Start: &start}).Validate(); err != nil {
		t.Errorf("Task with floating start: Validate() = %v, want nil", err)
	}
}

// TestValidateTaskOpenEnded asserts that a Task carrying neither due, start,
// nor estimatedDuration is valid (Section 5.2): an open-ended to-do is
// conformant, and the duration/due interplay is left lenient on a Task by
// design. Also exercises the combinations the spec permits.
func TestValidateTaskOpenEnded(t *testing.T) {
	t.Parallel()

	due := LocalDateTime{}
	start := LocalDateTime{}
	dur := mustDuration(t, "PT1H")

	cases := []struct {
		name string
		task *Task
	}{
		{"neither due nor start", &Task{Type: typeTask, UID: "x"}},
		{"due only", &Task{Type: typeTask, UID: "x", Due: &due}},
		{"start and estimatedDuration", &Task{
			Type: typeTask, UID: "x", Start: &start, EstimatedDuration: dur,
		}},
		{"due and start and estimatedDuration", &Task{
			Type: typeTask, UID: "x", Due: &due, Start: &start, EstimatedDuration: dur,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.task.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestValidateJoinsMultiple asserts that an object violating several MUSTs
// returns all of them: the joined error unwraps to each *ValidationError, and
// errors.As binds the first. An Event missing both uid and @type is the
// smallest two-violation case.
func TestValidateJoinsMultiple(t *testing.T) {
	t.Parallel()

	err := (&Event{Type: "Task"}).Validate() // wrong @type AND missing uid
	if err == nil {
		t.Fatal("Validate() = nil, want two violations")
	}

	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("multi-violation error %T does not Unwrap() []error", err)
	}
	parts := joined.Unwrap()
	if len(parts) != 2 {
		t.Fatalf("got %d violations, want 2: %v", len(parts), err)
	}

	gotProps := make(map[string]bool)
	for _, p := range parts {
		var verr *ValidationError
		if !errors.As(p, &verr) {
			t.Fatalf("joined part %v is not a *ValidationError", p)
		}
		gotProps[verr.Property] = true
	}
	for _, want := range []string{"@type", "uid"} {
		if !gotProps[want] {
			t.Errorf("missing violation for %q; got %v", want, gotProps)
		}
	}

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Error("errors.As against the joined error did not bind a *ValidationError")
	}
}

// TestValidateRecurrenceTimeZoneValid asserts the new phase-two checks accept
// conformant objects: a resolved custom time zone, an IANA time zone (never
// closure-checked), a well-formed override key with a nested patch pointer, and
// a rule bounded by exactly one of count/until. None of these is a violation, so
// adding the checks must not regress valid input.
func TestValidateRecurrenceTimeZoneValid(t *testing.T) {
	t.Parallel()

	count := uint(3)
	until := LocalDateTime{Year: 2020, Month: 12, Day: 31}

	cases := []struct {
		name  string
		event *Event
	}{
		{
			name: "custom timeZone resolved in timeZones",
			event: &Event{
				Type: typeEvent, UID: "x",
				TimeZone:  TimeZoneId("/custom"),
				TimeZones: map[TimeZoneId]TimeZone{"/custom": {TzID: "Etc/Custom"}},
			},
		},
		{
			name: "IANA timeZone needs no embedded definition",
			event: &Event{
				Type: typeEvent, UID: "x",
				TimeZone: TimeZoneId("America/New_York"),
			},
		},
		{
			name: "override key valid with nested pointer",
			event: &Event{
				Type: typeEvent, UID: "x",
				RecurrenceOverrides: map[string]PatchObject{
					"2020-01-08T14:00:00": {
						"title":                json.RawMessage(`"moved"`),
						"locations/1/name":     json.RawMessage(`"Room B"`),
						"participants/2/@type": json.RawMessage(`"Participant"`),
					},
				},
			},
		},
		{
			name: "rule bounded by count only",
			event: &Event{
				Type: typeEvent, UID: "x",
				RecurrenceRules: []RecurrenceRule{{Frequency: FrequencyWeekly, Count: &count}},
			},
		},
		{
			name: "rule bounded by until only",
			event: &Event{
				Type: typeEvent, UID: "x",
				RecurrenceRules: []RecurrenceRule{{Frequency: FrequencyWeekly, Until: &until}},
			},
		},
		{
			name: "localization removal pointer is valid",
			event: &Event{
				Type: typeEvent, UID: "x",
				Localizations: map[string]PatchObject{
					"fr": {"description": json.RawMessage(`null`)},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.event.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestValidateRecurrenceReportsAllViolations asserts the recurrence/time-zone
// checks join into the multi-violation contract: an Event that violates several
// of the new rules at once returns each as a *ValidationError, so a caller that
// iterates the joined error sees every problem rather than just the first.
func TestValidateRecurrenceReportsAllViolations(t *testing.T) {
	t.Parallel()

	count := uint(1)
	until := LocalDateTime{Year: 2021, Month: 1, Day: 1}
	err := (&Event{
		Type: typeEvent, UID: "x",
		TimeZone: TimeZoneId("/missing"),
		RecurrenceRules: []RecurrenceRule{{
			Frequency: FrequencyDaily, Count: &count, Until: &until,
		}},
		RecurrenceOverrides: map[string]PatchObject{
			"bad-key": {"@type": json.RawMessage(`"Task"`)},
		},
	}).Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want several violations")
	}

	want := map[string]bool{
		"timeZone":                          false,
		"recurrenceRules[0]":                false,
		"recurrenceOverrides/bad-key":       false,
		"recurrenceOverrides/bad-key/@type": false,
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("multi-violation error %T does not Unwrap() []error", err)
	}
	for _, part := range joined.Unwrap() {
		var verr *ValidationError
		if !errors.As(part, &verr) {
			t.Fatalf("joined part %v is not a *ValidationError", part)
		}
		if _, expected := want[verr.Property]; !expected {
			t.Errorf("unexpected violation property %q", verr.Property)
			continue
		}
		want[verr.Property] = true
	}
	for prop, seen := range want {
		if !seen {
			t.Errorf("missing expected violation for %q", prop)
		}
	}
}

// mustDuration parses a JSCalendar duration literal for a test or fails. It
// goes through the JSON codec so the test does not depend on the Duration
// constructor's internal shape.
func mustDuration(t *testing.T, s string) *Duration {
	t.Helper()
	var d Duration
	if err := json.Unmarshal([]byte(`"`+s+`"`), &d); err != nil {
		t.Fatalf("parse duration %q: %v", s, err)
	}
	return &d
}
