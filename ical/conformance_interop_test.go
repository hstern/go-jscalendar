// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

//go:build interop

// This file is the iCalendar round-trip conformance corpus, the acceptance gate
// for the phase-6 iCalendar conversion. It runs only under the "interop" build
// tag — the same tag the CI "interop" job selects (go test -tags=interop ./...)
// — so the core to_ical_test.go / from_ical_test.go unit tests stay fast and
// always-on while the broader corpus runs where it is meant to.
//
// # What "round trip" means here
//
// The corpus asserts a SEMANTIC round trip, not a byte-exact one. iCalendar
// property ordering and line folding are not significant, and FromICal
// canonicalizes equivalent forms (a DTEND becomes a duration, a Z-suffixed
// DATE-TIME becomes a wall clock plus the Etc/UTC zone). Comparing raw .ics
// bytes would fail on those insignificant differences. Instead each case runs
//
//	iCal --FromICal--> JSCalendar --ToICal--> iCal --FromICal--> JSCalendar
//
// and compares the two JSCalendar object sets by their byte-stable JSON
// encoding. The JSCalendar model is the normal form: if the second JSON equals
// the first, the iCalendar produced by ToICal carried exactly the information
// FromICal had extracted — a lossless semantic round trip over the mapped
// surface. The reverse direction (JSCalendar --ToICal--> iCal --FromICal-->
// JSCalendar) is checked by TestJSCalendarModelRoundTrip below.
//
// The lossy edges documented on ToICal are exercised by
// TestToICalLossyEdges, which pins them so a future change that narrows a lossy
// edge shows up as a test delta rather than passing silently.
package ical_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/ical"
)

// corpus lists the conformance .ics fixtures and the feature each pins. Every
// mapped construct in the calext correspondence appears in at least one entry.
var corpus = []struct {
	file    string
	feature string
}{
	{"event_zoned.ics", "a zoned timed event with DTEND-derived duration, RRULE, VALARM, and the common metadata"},
	{"event_allday.ics", "an all-day (VALUE=DATE) event with a whole-day span"},
	{"event_floating.ics", "a floating timed event (bare DATE-TIME, no Z, no TZID)"},
	{"event_recurring.ics", "a recurring event with an RRULE (UNTIL) and an EXRULE exclusion"},
	{"event_valarm.ics", "an event with two VALARMs (offset and RELATED=END triggers)"},
	{"todo.ics", "a VTODO with DUE, PERCENT-COMPLETE, and STATUS"},
	{"event_custom_vtimezone.ics", "an event referencing a custom VTIMEZONE with STANDARD/DAYLIGHT rules"},
}

// TestConformanceCorpusRoundTrip is the acceptance gate: every corpus fixture
// must survive the iCal -> JSCalendar -> iCal -> JSCalendar round trip with the
// JSCalendar model unchanged, and every intermediate object must Validate and
// re-encode as iCalendar.
func TestConformanceCorpusRoundTrip(t *testing.T) {
	t.Parallel()

	for _, tc := range corpus {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			first := loadAndConvert(t, tc.file)
			validateAll(t, first)

			// JSCalendar -> iCal -> re-decode -> JSCalendar.
			cal, err := ical.ToICal(first...)
			if err != nil {
				t.Fatalf("ToICal: %v", err)
			}
			text := mustEncode(t, cal)
			second, err := ical.FromICal(mustDecode(t, text))
			if err != nil {
				t.Fatalf("FromICal of round-tripped ics: %v", err)
			}

			if got, want := mustJSON(t, second), mustJSON(t, first); got != want {
				t.Errorf("round trip changed the JSCalendar model for %s\n (%s)\n first:  %s\n second: %s",
					tc.file, tc.feature, want, got)
			}
		})
	}
}

// TestJSCalendarModelRoundTrip checks the other direction: a JSCalendar object
// built by hand (covering the mapped surface) survives JSCalendar -> iCal ->
// JSCalendar with the typed fields the mapping covers preserved. It complements
// the iCal-anchored corpus, which can only exercise constructs an .ics file can
// express.
func TestJSCalendarModelRoundTrip(t *testing.T) {
	t.Parallel()

	nth := 1
	source := &jscalendar.Event{
		Type:        "Event",
		UID:         "model@example.com",
		Title:       "Planning",
		Description: "Sprint planning",
		Sequence:    2,
		Privacy:     "private",
		Status:      "confirmed",
		Updated:     &jscalendar.UTCDateTime{Year: 2026, Month: 1, Day: 2, Hour: 9, Minute: 30},
		Start:       &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
		TimeZone:    "America/New_York",
		Duration:    &jscalendar.Duration{Hours: 1, Minutes: 30},
		Keywords:    map[string]bool{"work": true},
		RecurrenceRules: []jscalendar.RecurrenceRule{{
			Frequency: jscalendar.FrequencyWeekly,
			ByDay:     []jscalendar.NDay{{Day: "mo", NthOfPeriod: &nth}},
			Count:     ptrUint(10),
		}},
		Alerts: map[jscalendar.Id]jscalendar.Alert{
			"1": {Action: "display", Trigger: jscalendar.NewTrigger(jscalendar.OffsetTrigger{
				Offset: jscalendar.SignedDuration{Negative: true, Duration: jscalendar.Duration{Minutes: 15}},
			})},
		},
	}

	cal, err := ical.ToICal(source)
	if err != nil {
		t.Fatalf("ToICal: %v", err)
	}
	objs, err := ical.FromICal(mustDecode(t, mustEncode(t, cal)))
	if err != nil {
		t.Fatalf("FromICal: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("got %d objects, want 1", len(objs))
	}
	got, ok := objs[0].(*jscalendar.Event)
	if !ok {
		t.Fatalf("got %T, want *jscalendar.Event", objs[0])
	}

	// The mapped fields must survive verbatim. ACTION lower-cases on the way out
	// and back, the duration is preserved, the recurrence and alert reconstruct.
	if got.UID != source.UID || got.Title != source.Title || got.Description != source.Description {
		t.Errorf("metadata changed: %+v", got)
	}
	if got.Sequence != source.Sequence || got.Privacy != source.Privacy || got.Status != source.Status {
		t.Errorf("scheduling metadata changed: seq=%d privacy=%q status=%q", got.Sequence, got.Privacy, got.Status)
	}
	if got.Start == nil || *got.Start != *source.Start || got.TimeZone != source.TimeZone {
		t.Errorf("start/zone changed: start=%v zone=%q", got.Start, got.TimeZone)
	}
	if got.Duration == nil || got.Duration.String() != source.Duration.String() {
		t.Errorf("duration changed: %v", got.Duration)
	}
	if len(got.RecurrenceRules) != 1 || got.RecurrenceRules[0].Frequency != jscalendar.FrequencyWeekly {
		t.Errorf("recurrence changed: %+v", got.RecurrenceRules)
	}
	if got.RecurrenceRules[0].Count == nil || *got.RecurrenceRules[0].Count != 10 {
		t.Errorf("recurrence count changed: %v", got.RecurrenceRules[0].Count)
	}
	if len(got.Alerts) != 1 {
		t.Fatalf("alerts changed: %+v", got.Alerts)
	}
	offset, ok := got.Alerts["1"].Trigger.Value().(jscalendar.OffsetTrigger)
	if !ok || offset.Offset.String() != "-PT15M" {
		t.Errorf("alert trigger changed: %+v", got.Alerts["1"].Trigger.Value())
	}
}

// TestToICalLossyEdges pins the documented lossy edges so a future change that
// narrows one is a visible test delta rather than a silent behavior change.
func TestToICalLossyEdges(t *testing.T) {
	t.Parallel()

	t.Run("multiple locations collapse to one", func(t *testing.T) {
		t.Parallel()
		event := &jscalendar.Event{
			Type: "Event", UID: "e",
			Locations: map[jscalendar.Id]jscalendar.Location{
				"1": {Type: "Location", Name: "Room A"},
				"2": {Type: "Location", Name: "Room B"},
			},
		}
		vevent := onlyComponent(t, event, goical.CompEvent)
		if len(vevent.Props.Values(goical.PropLocation)) != 1 {
			t.Errorf("LOCATION count = %d, want 1 (the lowest id wins; rest are lossy)",
				len(vevent.Props.Values(goical.PropLocation)))
		}
		if got, _ := vevent.Props.Text(goical.PropLocation); got != "Room A" {
			t.Errorf("LOCATION = %q, want Room A (lowest id)", got)
		}
	})

	t.Run("display alarm gains a synthesized description", func(t *testing.T) {
		t.Parallel()
		event := &jscalendar.Event{
			Type: "Event", UID: "e",
			Alerts: map[jscalendar.Id]jscalendar.Alert{
				"1": {Action: "display", Trigger: jscalendar.NewTrigger(jscalendar.OffsetTrigger{
					Offset: jscalendar.SignedDuration{Negative: true, Duration: jscalendar.Duration{Minutes: 5}},
				})},
			},
		}
		valarm := onlyComponent(t, event, goical.CompEvent).Children[0]
		if got, _ := valarm.Props.Text(goical.PropDescription); got != "Reminder" {
			t.Errorf("VALARM DESCRIPTION = %q, want the synthesized \"Reminder\"", got)
		}
	})

	t.Run("color and extra members are not emitted", func(t *testing.T) {
		t.Parallel()
		event := &jscalendar.Event{
			Type: "Event", UID: "e",
			Color: "turquoise",
			Extra: map[string]json.RawMessage{"x-vendor": json.RawMessage(`"v"`)},
		}
		vevent := onlyComponent(t, event, goical.CompEvent)
		// COLOR is a valid iCalendar property but is intentionally not part of the
		// mapping; its absence is the pinned lossy edge.
		if vevent.Props.Get(goical.PropColor) != nil {
			t.Error("COLOR was emitted; the mapping does not carry color")
		}
	})
}

// loadAndConvert reads a corpus .ics file and converts it with FromICal.
func loadAndConvert(t *testing.T, file string) []any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	objs, err := ical.FromICal(mustDecode(t, string(data)))
	if err != nil {
		t.Fatalf("FromICal %s: %v", file, err)
	}
	if len(objs) == 0 {
		t.Fatalf("%s converted to no objects", file)
	}
	return objs
}

// validateAll asserts every converted object passes the parent package's opt-in
// Validate, the correctness precondition for round-trip equality to be
// meaningful.
func validateAll(t *testing.T, objs []any) {
	t.Helper()
	for i, obj := range objs {
		switch v := obj.(type) {
		case *jscalendar.Event:
			if err := v.Validate(); err != nil {
				t.Errorf("object %d (Event) Validate: %v", i, err)
			}
		case *jscalendar.Task:
			if err := v.Validate(); err != nil {
				t.Errorf("object %d (Task) Validate: %v", i, err)
			}
		default:
			t.Errorf("object %d unexpected type %T", i, obj)
		}
	}
}

// onlyComponent converts a single object and returns its sole component of the
// named kind.
func onlyComponent(t *testing.T, obj any, name string) *goical.Component {
	t.Helper()
	cal, err := ical.ToICal(obj)
	if err != nil {
		t.Fatalf("ToICal: %v", err)
	}
	for _, child := range cal.Children {
		if child.Name == name {
			return child
		}
	}
	t.Fatalf("no %s component", name)
	return nil
}

// mustDecode parses iCalendar text into a *goical.Calendar.
func mustDecode(t *testing.T, text string) *goical.Calendar {
	t.Helper()
	cal, err := goical.NewDecoder(strings.NewReader(text)).Decode()
	if err != nil {
		t.Fatalf("decode ics: %v", err)
	}
	return cal
}

// mustEncode serializes a calendar to iCalendar text.
func mustEncode(t *testing.T, cal *goical.Calendar) string {
	t.Helper()
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		t.Fatalf("encode ics: %v", err)
	}
	return buf.String()
}

// mustJSON marshals a JSCalendar object set to its byte-stable JSON, the normal
// form the round-trip comparison uses.
func mustJSON(t *testing.T, objs []any) string {
	t.Helper()
	data, err := json.Marshal(objs)
	if err != nil {
		t.Fatalf("marshal objects: %v", err)
	}
	return string(data)
}

// ptrUint returns a pointer to a uint, for optional recurrence fields.
func ptrUint(v uint) *uint { return &v }
