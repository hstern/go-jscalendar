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

// This file gives the recurrenceOverrides / recurrenceId / recurrenceIdTimeZone
// wiring on Event and Task (RFC 8984, Section 4.3.2 and 4.3.5) dedicated
// round-trip coverage, the companion to recurrence_wiring_test.go's coverage of
// the recurrence rules. The broad Section 6 corpus test in parse_test.go
// already round-trips the 6.9 overrides figure as part of the byte-stable
// acceptance gate, but it asserts on the whole object; the tests here pin the
// override-specific behavior — the PatchObject values survive verbatim, the
// excluded:true deletion patch round-trips, a null-removal pointer survives as
// the §3.3 removal sentinel, and the map is keyed by LocalDateTime strings — so
// a regression in the override wiring fails with an override-named test rather
// than only tripping the corpus gate.

// TestRecurrenceOverridesCorpusRoundTrip drives the 6.9 recurring-event-with-
// overrides figure through Parse and re-marshal, asserting both byte-stability
// and that the overrides actually populated the typed RecurrenceOverrides field
// (not the Extra open-extension seam). It then inspects the decoded patches:
// the figure carries three overrides keyed by the overridden occurrences'
// LocalDateTime starts, one of which is the excluded:true deletion patch
// (Section 4.3.5). The byte assertion guards the wire form; the field and patch
// assertions guard against a codec that reproduces the bytes via Extra without
// ever populating the typed map.
func TestRecurrenceOverridesCorpusRoundTrip(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(rfc8984CorpusDir, "6.9-recurring-event-with-overrides.json"))
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

	// The overrides landed in the typed field, not the Extra seam.
	if got := len(ev.RecurrenceOverrides); got != 3 {
		t.Fatalf("RecurrenceOverrides length = %d, want 3", got)
	}
	if _, captured := ev.Extra["recurrenceOverrides"]; captured {
		t.Errorf("recurrenceOverrides captured in Extra; want the typed field")
	}

	// The map is keyed by the overridden occurrences' LocalDateTime starts, in
	// the master's Europe/London zone (the figure's timeZone).
	const excludedKey = "2020-04-01T09:00:00"
	patch, ok := ev.RecurrenceOverrides[excludedKey]
	if !ok {
		t.Fatalf("missing override for key %q", excludedKey)
	}
	// That occurrence is deleted via the excluded:true patch (Section 4.3.5):
	// the value is a literal true, not the §3.3 null-removal sentinel, so it
	// reads as a normal patch value rather than a removal.
	if patch.IsRemoval("excluded") {
		t.Errorf("excluded patch reported as a null removal; want a literal true value")
	}
	if got := string(patch["excluded"]); got != "true" {
		t.Errorf("override[%q][excluded] = %q, want %q", excludedKey, got, "true")
	}

	got, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != canonical {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, canonical)
	}
}

// TestRecurrenceOverridesConstructedRoundTrip pins a constructed Event whose
// RecurrenceOverrides map is keyed by LocalDateTime strings and whose patch
// values exercise the two distinct override mechanisms: a property patch, the
// excluded:true whole-occurrence deletion, and a JSON-null removal pointer (the
// §3.3 removal sentinel). The byte-stable assertion confirms the codec emits
// the map in sorted-key order with each PatchObject's members sorted, so a
// caller pinning exact bytes is undisturbed; the round trip therefore reflects
// the library's canonical form, not the construction order.
func TestRecurrenceOverridesConstructedRoundTrip(t *testing.T) {
	t.Parallel()

	// Three overrides keyed by occurrence-start LocalDateTimes: one retitles an
	// occurrence, one deletes it via excluded:true, one removes a property with
	// the null sentinel. Map keys and each PatchObject's pointers are written in
	// sorted order so the construction matches the canonical marshaled form and
	// the round trip is exact.
	ev := &Event{
		Type:     "Event",
		UID:      "override-construct-1",
		Start:    mustLocalDateTime(t, "2020-01-06T09:00:00"),
		TimeZone: "America/New_York",
		RecurrenceRules: []RecurrenceRule{
			{Frequency: FrequencyWeekly},
		},
		RecurrenceOverrides: map[string]PatchObject{
			"2020-01-13T09:00:00": {"title": json.RawMessage(`"Special session"`)},
			"2020-01-20T09:00:00": {"excluded": json.RawMessage(`true`)},
			"2020-01-27T09:00:00": {"color": json.RawMessage(`null`)},
		},
	}

	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	const want = `{"@type":"Event","uid":"override-construct-1",` +
		`"recurrenceRules":[{"@type":"RecurrenceRule","frequency":"weekly"}],` +
		`"recurrenceOverrides":{` +
		`"2020-01-13T09:00:00":{"title":"Special session"},` +
		`"2020-01-20T09:00:00":{"excluded":true},` +
		`"2020-01-27T09:00:00":{"color":null}},` +
		`"start":"2020-01-06T09:00:00","timeZone":"America/New_York"}`
	if string(out) != want {
		t.Fatalf("marshal mismatch\n got: %s\nwant: %s", out, want)
	}

	// Re-parse and assert the typed field repopulated, the excluded deletion is
	// a literal true (not a removal), and the null pointer reads as a removal.
	obj, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	back, ok := obj.(*Event)
	if !ok {
		t.Fatalf("Parse = %T, want *Event", obj)
	}
	if got := len(back.RecurrenceOverrides); got != 3 {
		t.Fatalf("round-tripped RecurrenceOverrides length = %d, want 3", got)
	}
	if back.RecurrenceOverrides["2020-01-20T09:00:00"].IsRemoval("excluded") {
		t.Errorf("excluded:true reported as a removal; want a literal true value")
	}
	if !back.RecurrenceOverrides["2020-01-27T09:00:00"].IsRemoval("color") {
		t.Errorf("color:null not reported as a removal; want the §3.3 removal sentinel")
	}

	roundTrip, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("re-Marshal: %v", err)
	}
	if string(roundTrip) != want {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", roundTrip, want)
	}
}

// TestRecurrenceIDStandaloneRoundTrip covers the standalone-override form: an
// Event that is itself one expanded occurrence, carrying recurrenceId and
// recurrenceIdTimeZone (RFC 8984, Section 4.3.2) to point back at the master
// occurrence it replaces, rather than living inside a master's
// recurrenceOverrides map. Both members must round-trip byte-stably and
// repopulate their typed fields. No Section 6 figure exercises the standalone
// form, so the input is constructed here.
func TestRecurrenceIDStandaloneRoundTrip(t *testing.T) {
	t.Parallel()

	const input = `{"@type":"Event","uid":"standalone-override-1",` +
		`"recurrenceId":"2020-04-01T09:00:00","recurrenceIdTimeZone":"Europe/London",` +
		`"start":"2020-04-01T10:00:00","duration":"PT2H","timeZone":"Europe/London"}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ev, ok := obj.(*Event)
	if !ok {
		t.Fatalf("Parse = %T, want *Event", obj)
	}

	if ev.RecurrenceID == nil {
		t.Fatalf("RecurrenceID = nil, want 2020-04-01T09:00:00")
	}
	if got := ev.RecurrenceID.String(); got != "2020-04-01T09:00:00" {
		t.Errorf("RecurrenceID = %q, want %q", got, "2020-04-01T09:00:00")
	}
	if got := ev.RecurrenceIDTimeZone; got != "Europe/London" {
		t.Errorf("RecurrenceIDTimeZone = %q, want %q", got, "Europe/London")
	}

	got, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != input {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, input)
	}
}

// TestTaskRecurrenceOverridesRoundTrip mirrors the Event override coverage for
// Task: recurrenceOverrides, recurrenceId, and recurrenceIdTimeZone are
// common-property members (RFC 8984, Section 4.3 applies to both top-level
// scheduled types), so a Task carrying an override map and the standalone
// pointers must round-trip identically. Without this, a wiring regression on
// Task alone would slip past the Event-only corpus figures.
func TestTaskRecurrenceOverridesRoundTrip(t *testing.T) {
	t.Parallel()

	const input = `{"@type":"Task","uid":"task-override-1",` +
		`"recurrenceId":"2020-02-01T09:00:00","recurrenceIdTimeZone":"America/New_York",` +
		`"recurrenceOverrides":{"2020-02-08T09:00:00":{"excluded":true}},` +
		`"due":"2020-02-01T17:00:00","start":"2020-02-01T09:00:00"}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tk, ok := obj.(*Task)
	if !ok {
		t.Fatalf("Parse = %T, want *Task", obj)
	}

	if tk.RecurrenceID == nil || tk.RecurrenceID.String() != "2020-02-01T09:00:00" {
		t.Errorf("RecurrenceID = %v, want 2020-02-01T09:00:00", tk.RecurrenceID)
	}
	if tk.RecurrenceIDTimeZone != "America/New_York" {
		t.Errorf("RecurrenceIDTimeZone = %q, want %q", tk.RecurrenceIDTimeZone, "America/New_York")
	}
	if got := len(tk.RecurrenceOverrides); got != 1 {
		t.Fatalf("RecurrenceOverrides length = %d, want 1", got)
	}

	got, err := json.Marshal(tk)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != input {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, input)
	}
}

// TestRecurrenceOverridesFieldTags is a compile-and-encode guard on the JSON
// tags the override wiring depends on: recurrenceOverrides, recurrenceId, and
// recurrenceIdTimeZone must use exactly those member names and must be omitted
// when empty (omitempty). A renamed tag or a dropped omitempty would silently
// break interop, so this test asserts both the empty-object omission and the
// populated member names directly rather than inferring them from a larger
// round trip.
func TestRecurrenceOverridesFieldTags(t *testing.T) {
	t.Parallel()

	members := []string{"recurrenceOverrides", "recurrenceId", "recurrenceIdTimeZone"}

	// Empty fields must omit every member entirely.
	empty, err := json.Marshal(&Event{Type: "Event", UID: "e"})
	if err != nil {
		t.Fatalf("Marshal empty: %v", err)
	}
	for _, member := range members {
		if strings.Contains(string(empty), member) {
			t.Errorf("empty Event marshaled %q; want it omitted: %s", member, empty)
		}
	}

	// Populated fields must use exactly those member names.
	filled, err := json.Marshal(&Event{
		Type:                 "Event",
		UID:                  "e",
		RecurrenceID:         mustLocalDateTime(t, "2020-01-01T09:00:00"),
		RecurrenceIDTimeZone: "Europe/London",
		RecurrenceOverrides: map[string]PatchObject{
			"2020-01-08T09:00:00": {"excluded": json.RawMessage(`true`)},
		},
	})
	if err != nil {
		t.Fatalf("Marshal filled: %v", err)
	}
	for _, member := range members {
		if !strings.Contains(string(filled), `"`+member+`"`) {
			t.Errorf("filled Event missing member %q: %s", member, filled)
		}
	}
}

// mustLocalDateTime parses a JSCalendar LocalDateTime string for use in a test
// fixture, failing the test on a malformed value rather than returning an
// error to the caller.
func mustLocalDateTime(t *testing.T, s string) *LocalDateTime {
	t.Helper()
	ldt, err := ParseLocalDateTime(s)
	if err != nil {
		t.Fatalf("parse LocalDateTime %q: %v", s, err)
	}
	return &ldt
}
