// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file gives the recurrenceRules / excludedRecurrenceRules wiring on
// Event and Task (RFC 8984, Section 4.3.3–4.3.4) dedicated round-trip
// coverage. The broad Section 6 corpus test in parse_test.go already exercises
// the recurring figures as part of the byte-stable acceptance gate, but it
// asserts on the whole object; the tests here pin the recurrence fields
// specifically — that a recurringRules slice survives Parse → re-marshal, and
// that excludedRecurrenceRules round-trips alongside it — so a regression in
// the recurrence wiring fails with a recurrence-named test rather than only
// tripping the corpus gate.

// TestRecurrenceRulesCorpusRoundTrip drives the two recurring-event figures
// that carry only a recurrenceRules member — 6.4 (all-day, yearly) and 6.7
// (floating-time, daily) — through Parse and re-marshal, asserting both that
// the bytes are stable and that the decoded Event actually carries the parsed
// rule. The byte assertion guards the wire form; the field assertion guards
// against a codec that reproduces the bytes via the Extra open-extension seam
// without the typed RecurrenceRules field ever being populated.
func TestRecurrenceRulesCorpusRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file          string
		wantFrequency Frequency
	}{
		{"6.4-all-day-event.json", FrequencyYearly},
		{"6.7-floating-time-event.json", FrequencyDaily},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(rfc8984CorpusDir, tc.file))
			if err != nil {
				t.Fatalf("read corpus: %v", err)
			}
			canonical := trimTrailingNewline(string(raw))

			obj, err := Parse([]byte(canonical))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ev, ok := obj.(*Event)
			if !ok {
				t.Fatalf("Parse = %T, want *Event", obj)
			}

			// The rule landed in the typed field, not the Extra seam.
			if len(ev.RecurrenceRules) != 1 {
				t.Fatalf("RecurrenceRules length = %d, want 1", len(ev.RecurrenceRules))
			}
			if got := ev.RecurrenceRules[0].Frequency; got != tc.wantFrequency {
				t.Errorf("RecurrenceRules[0].Frequency = %q, want %q", got, tc.wantFrequency)
			}
			// These figures carry no exclusions; the field must stay nil so an
			// empty member is never emitted.
			if ev.ExcludedRecurrenceRules != nil {
				t.Errorf("ExcludedRecurrenceRules = %v, want nil", ev.ExcludedRecurrenceRules)
			}

			got, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != canonical {
				t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, canonical)
			}
		})
	}
}

// TestExcludedRecurrenceRulesRoundTrip pins that an Event carrying BOTH
// recurrenceRules and excludedRecurrenceRules round-trips byte-stably and
// repopulates both typed fields. No RFC 8984 Section 6 figure exercises
// excludedRecurrenceRules, so the input is constructed here. The semantics the
// fields document — excluded rules subtract from the produced set, both
// evaluated against the master start — are a consumer's expansion concern; this
// test asserts only the wiring the library is responsible for: the rules
// survive the codec verbatim, in declaration order, with excludedRecurrenceRules
// emitted after recurrenceRules.
func TestExcludedRecurrenceRulesRoundTrip(t *testing.T) {
	t.Parallel()

	// A weekly event that recurs every week but skips the first week of each
	// month: recurrenceRules produces the weekly set, excludedRecurrenceRules
	// removes the monthly first-week instance. Both reference the same Start.
	const input = `{"@type":"Event","uid":"rec-excl-1",` +
		`"recurrenceRules":[{"@type":"RecurrenceRule","frequency":"weekly"}],` +
		`"excludedRecurrenceRules":[{"@type":"RecurrenceRule","frequency":"monthly"}],` +
		`"start":"2020-01-06T09:00:00","duration":"PT1H","timeZone":"America/New_York"}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ev, ok := obj.(*Event)
	if !ok {
		t.Fatalf("Parse = %T, want *Event", obj)
	}

	if len(ev.RecurrenceRules) != 1 || ev.RecurrenceRules[0].Frequency != FrequencyWeekly {
		t.Errorf("RecurrenceRules = %+v, want one weekly rule", ev.RecurrenceRules)
	}
	if len(ev.ExcludedRecurrenceRules) != 1 || ev.ExcludedRecurrenceRules[0].Frequency != FrequencyMonthly {
		t.Errorf("ExcludedRecurrenceRules = %+v, want one monthly rule", ev.ExcludedRecurrenceRules)
	}

	got, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Declaration order puts recurrenceRules before excludedRecurrenceRules,
	// which the constructed input already matches, so the round trip is exact.
	if string(got) != input {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, input)
	}
}

// TestTaskRecurrenceRulesRoundTrip mirrors the Event coverage for Task: the
// recurrence wiring lives on both top-level scheduled types (RFC 8984,
// Section 4.3 is a common-property section), and a Task carrying both rule
// kinds must round-trip identically. Without this, a wiring regression on Task
// alone would slip past the Event-only corpus figures.
func TestTaskRecurrenceRulesRoundTrip(t *testing.T) {
	t.Parallel()

	const input = `{"@type":"Task","uid":"task-rec-1",` +
		`"recurrenceRules":[{"@type":"RecurrenceRule","frequency":"daily"}],` +
		`"excludedRecurrenceRules":[{"@type":"RecurrenceRule","frequency":"weekly"}],` +
		`"due":"2020-01-01T17:00:00","start":"2020-01-01T09:00:00"}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tk, ok := obj.(*Task)
	if !ok {
		t.Fatalf("Parse = %T, want *Task", obj)
	}

	if len(tk.RecurrenceRules) != 1 || tk.RecurrenceRules[0].Frequency != FrequencyDaily {
		t.Errorf("RecurrenceRules = %+v, want one daily rule", tk.RecurrenceRules)
	}
	if len(tk.ExcludedRecurrenceRules) != 1 || tk.ExcludedRecurrenceRules[0].Frequency != FrequencyWeekly {
		t.Errorf("ExcludedRecurrenceRules = %+v, want one weekly rule", tk.ExcludedRecurrenceRules)
	}

	got, err := json.Marshal(tk)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != input {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, input)
	}
}

// TestRecurrenceRulesFieldTags is a compile-and-encode guard on the JSON tags
// the wiring depends on: recurrenceRules and excludedRecurrenceRules must use
// exactly those member names and must be omitted when empty (omitempty). A
// renamed tag or a dropped omitempty would silently break interop, so this test
// asserts both the present-member names and the empty-object omission directly
// rather than inferring them from a larger round trip.
func TestRecurrenceRulesFieldTags(t *testing.T) {
	t.Parallel()

	// Empty rules must omit both members entirely.
	empty, err := json.Marshal(&Event{Type: "Event", UID: "e"})
	if err != nil {
		t.Fatalf("Marshal empty: %v", err)
	}
	for _, member := range []string{"recurrenceRules", "excludedRecurrenceRules"} {
		if strings.Contains(string(empty), member) {
			t.Errorf("empty Event marshaled %q; want it omitted: %s", member, empty)
		}
	}

	// Populated rules must use exactly those member names.
	count := uint(0)
	filled, err := json.Marshal(&Event{
		Type:                    "Event",
		UID:                     "e",
		RecurrenceRules:         []RecurrenceRule{{Frequency: FrequencyDaily, Count: &count}},
		ExcludedRecurrenceRules: []RecurrenceRule{{Frequency: FrequencyWeekly}},
	})
	if err != nil {
		t.Fatalf("Marshal filled: %v", err)
	}
	for _, member := range []string{`"recurrenceRules"`, `"excludedRecurrenceRules"`} {
		if !strings.Contains(string(filled), member) {
			t.Errorf("filled Event missing member %s: %s", member, filled)
		}
	}
}
