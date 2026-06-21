// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	jscalendar "github.com/hstern/go-jscalendar"
)

// loadEvent decodes a testdata RFC 8984 figure into an Event.
func loadEvent(t *testing.T, name string) *jscalendar.Event {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "rfc8984", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var e jscalendar.Event
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return &e
}

// local builds an instant from wall-clock fields in a named IANA zone.
func local(t *testing.T, zone, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatalf("load zone %s: %v", zone, err)
	}
	lt, err := jscalendar.ParseLocalDateTime(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return localToTime(lt, loc)
}

// recurrenceIDs collects the recurrenceId (or, for non-recurring occurrences,
// the start) of each occurrence as a canonical string for order-sensitive
// assertions.
func recurrenceIDs(occ []*jscalendar.Event) []string {
	out := make([]string, len(occ))
	for i, e := range occ {
		switch {
		case e.RecurrenceID != nil:
			out[i] = e.RecurrenceID.String()
		case e.Start != nil:
			out[i] = e.Start.String()
		}
	}
	return out
}

func uintPtr(v uint) *uint { return &v }

func mustLocal(t *testing.T, s string) jscalendar.LocalDateTime {
	t.Helper()
	lt, err := jscalendar.ParseLocalDateTime(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return lt
}

func TestOccurrencesDailyCount(t *testing.T) {
	start := mustLocal(t, "2020-01-01T07:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "America/New_York",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Count: uintPtr(3)},
		},
	}
	occ, err := Occurrences(e, local(t, "America/New_York", "2020-01-01T00:00:00"),
		local(t, "America/New_York", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	got := recurrenceIDs(occ)
	want := []string{"2020-01-01T07:00:00", "2020-01-02T07:00:00", "2020-01-03T07:00:00"}
	if len(got) != len(want) {
		t.Fatalf("got %d occurrences %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
	for _, o := range occ {
		if o.RecurrenceIDTimeZone != "America/New_York" {
			t.Errorf("recurrenceIdTimeZone = %q, want America/New_York", o.RecurrenceIDTimeZone)
		}
		if len(o.RecurrenceRules) != 0 {
			t.Errorf("occurrence should not carry recurrenceRules")
		}
		if err := o.Validate(); err != nil {
			t.Errorf("occurrence does not validate: %v", err)
		}
	}
}

func TestOccurrencesWeeklyInterval(t *testing.T) {
	start := mustLocal(t, "2020-01-01T09:00:00") // Wednesday
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyWeekly, Interval: 2, Count: uintPtr(3)},
		},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2020-03-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2020-01-01T09:00:00", "2020-01-15T09:00:00", "2020-01-29T09:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != 3 {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesMonthlyByDayPositional(t *testing.T) {
	start := mustLocal(t, "2020-01-26T10:00:00") // last Sunday of Jan 2020
	nth := -1
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{{
			Frequency: jscalendar.FrequencyMonthly,
			ByDay:     []jscalendar.NDay{{Day: "su", NthOfPeriod: &nth}},
			Count:     uintPtr(3),
		}},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2020-12-31T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	// Last Sundays: Jan 26, Feb 23, Mar 29 (2020).
	want := []string{"2020-01-26T10:00:00", "2020-02-23T10:00:00", "2020-03-29T10:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != 3 {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesBySetPositionLastWeekday(t *testing.T) {
	start := mustLocal(t, "2020-01-31T08:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{{
			Frequency: jscalendar.FrequencyMonthly,
			ByDay: []jscalendar.NDay{
				{Day: "mo"}, {Day: "tu"}, {Day: "we"}, {Day: "th"}, {Day: "fr"},
			},
			BySetPosition: []int{-1},
			Count:         uintPtr(3),
		}},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2020-12-31T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	// Last weekday of month: Jan 31 (Fri), Feb 28 (Fri), Mar 31 (Tue) 2020.
	want := []string{"2020-01-31T08:00:00", "2020-02-28T08:00:00", "2020-03-31T08:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != 3 {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesUntilTermination(t *testing.T) {
	start := mustLocal(t, "2020-01-01T12:00:00")
	until := mustLocal(t, "2020-01-04T12:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Until: &until},
		},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	// Until is inclusive: Jan 1,2,3,4.
	want := []string{
		"2020-01-01T12:00:00", "2020-01-02T12:00:00",
		"2020-01-03T12:00:00", "2020-01-04T12:00:00",
	}
	got := recurrenceIDs(occ)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesExcludedRule(t *testing.T) {
	start := mustLocal(t, "2020-01-01T09:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Count: uintPtr(6)},
		},
		// Exclude every other day starting from the master start.
		ExcludedRecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Interval: 2},
		},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2020-02-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	// Daily Jan1..Jan6 minus Jan1,3,5 leaves Jan2,4,6.
	want := []string{"2020-01-02T09:00:00", "2020-01-04T09:00:00", "2020-01-06T09:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesWindowClipping(t *testing.T) {
	start := mustLocal(t, "2020-01-01T00:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{{Frequency: jscalendar.FrequencyDaily}},
	}
	// Half-open [Jan 3, Jan 6): Jan 3, 4, 5 — Jan 6 excluded at the boundary.
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-03T00:00:00"),
		local(t, "UTC", "2020-01-06T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2020-01-03T00:00:00", "2020-01-04T00:00:00", "2020-01-05T00:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesYearlyFigure64(t *testing.T) {
	e := loadEvent(t, "6.4-all-day-event.json")
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2023-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2020-04-01T00:00:00", "2021-04-01T00:00:00", "2022-04-01T00:00:00"}
	got := recurrenceIDs(occ)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("occurrence %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestOccurrencesFloatingFigure67(t *testing.T) {
	e := loadEvent(t, "6.7-floating-time-event.json") // daily 07:00, no time zone
	if e.TimeZone != "" {
		t.Fatalf("figure 6.7 should be floating, got timeZone %q", e.TimeZone)
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2020-01-04T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(occ) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(occ))
	}
	for _, o := range occ {
		if o.RecurrenceIDTimeZone != "" {
			t.Errorf("floating occurrence must not carry recurrenceIdTimeZone, got %q", o.RecurrenceIDTimeZone)
		}
		if o.RecurrenceID == nil {
			t.Errorf("recurring occurrence should carry recurrenceId")
		}
	}
}

func TestOccurrencesNonRecurringSingleInstance(t *testing.T) {
	start := mustLocal(t, "2020-05-05T10:00:00")
	e := &jscalendar.Event{Type: "Event", UID: "u", Start: &start, TimeZone: "UTC"}

	in, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(in) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(in))
	}
	if in[0].RecurrenceID != nil {
		t.Errorf("non-recurring occurrence must not carry recurrenceId")
	}

	out, err := Occurrences(e, local(t, "UTC", "2021-01-01T00:00:00"),
		local(t, "UTC", "2022-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("start outside window: got %d occurrences, want 0", len(out))
	}
}

func TestOccurrencesOverridesFigure69(t *testing.T) {
	e := loadEvent(t, "6.9-recurring-event-with-overrides.json")
	occ, err := Occurrences(e, local(t, "Europe/London", "2020-01-01T00:00:00"),
		local(t, "Europe/London", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]*jscalendar.Event{}
	for _, o := range occ {
		byID[o.RecurrenceID.String()] = o
		if err := o.Validate(); err != nil {
			t.Errorf("occurrence %s does not validate: %v", o.RecurrenceID, err)
		}
	}

	// Override that ADDS an instance the rule does not produce (a Tuesday) and
	// changes its title.
	added := byID["2020-01-07T14:00:00"]
	if added == nil {
		t.Fatalf("override-added occurrence 2020-01-07T14:00:00 missing")
	}
	if added.Title != "Introduction to Calculus I (optional)" {
		t.Errorf("added title = %q", added.Title)
	}
	if added.Start.String() != "2020-01-07T14:00:00" {
		t.Errorf("added start = %s, want 2020-01-07T14:00:00", added.Start)
	}

	// Override with {"excluded":true} DELETES this instance.
	if _, ok := byID["2020-04-01T09:00:00"]; ok {
		t.Errorf("excluded occurrence 2020-04-01T09:00:00 should be absent")
	}

	// Override that MODIFIES (and moves the start of) the exam instance, which
	// is itself past the rule's "until" — an added, patched occurrence.
	exam := byID["2020-06-25T09:00:00"]
	if exam == nil {
		t.Fatalf("override exam occurrence 2020-06-25T09:00:00 missing")
	}
	if exam.Title != "Calculus I Exam" {
		t.Errorf("exam title = %q", exam.Title)
	}
	if exam.Start.String() != "2020-06-25T10:00:00" {
		t.Errorf("exam start = %s, want moved to 2020-06-25T10:00:00", exam.Start)
	}
	if exam.Duration == nil || exam.Duration.String() != "PT2H" {
		t.Errorf("exam duration = %v, want PT2H", exam.Duration)
	}
	if _, ok := exam.Locations["auditorium"]; !ok {
		t.Errorf("exam should carry the patched auditorium location, got %v", exam.Locations)
	}
	if _, ok := exam.Locations["mlab"]; ok {
		t.Errorf("exam locations should have been replaced, mlab still present")
	}

	// A plain rule occurrence keeps the master title and zone.
	plain := byID["2020-01-08T09:00:00"]
	if plain == nil {
		t.Fatalf("plain occurrence 2020-01-08T09:00:00 missing")
	}
	if plain.Title != "Calculus I" {
		t.Errorf("plain title = %q, want Calculus I", plain.Title)
	}
	if plain.RecurrenceIDTimeZone != "Europe/London" {
		t.Errorf("plain recurrenceIdTimeZone = %q", plain.RecurrenceIDTimeZone)
	}
	if _, ok := plain.Locations["mlab"]; !ok {
		t.Errorf("plain occurrence should keep the master mlab location")
	}
}

func TestOccurrencesNestedParticipantOverrideFigure610(t *testing.T) {
	e := loadEvent(t, "6.10-recurring-event-with-participants.json")
	occ, err := Occurrences(e, local(t, "Africa/Johannesburg", "2020-03-01T00:00:00"),
		local(t, "Africa/Johannesburg", "2020-03-31T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	var patched *jscalendar.Event
	for _, o := range occ {
		if o.RecurrenceID.String() == "2020-03-04T09:00:00" {
			patched = o
		}
	}
	if patched == nil {
		t.Fatalf("expected an occurrence on 2020-03-04T09:00:00")
	}
	p, ok := patched.Participants["dG9tQGZvb2Jhci5xlLmNvbQ"]
	if !ok {
		t.Fatalf("participant missing on patched occurrence")
	}
	if p.ParticipationStatus != "declined" {
		t.Errorf("patched participationStatus = %q, want declined", p.ParticipationStatus)
	}
}

func TestOccurrencesDSTKeepsWallClock(t *testing.T) {
	start := mustLocal(t, "2020-03-07T09:00:00") // before US spring-forward (Mar 8)
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "America/New_York",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Count: uintPtr(4)},
		},
	}
	occ, err := Occurrences(e, local(t, "America/New_York", "2020-03-01T00:00:00"),
		local(t, "America/New_York", "2020-04-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(occ) != 4 {
		t.Fatalf("got %d occurrences, want 4", len(occ))
	}
	for _, o := range occ {
		if o.Start.Hour != 9 || o.Start.Minute != 0 {
			t.Errorf("occurrence %s does not keep 09:00 wall clock", o.Start)
		}
	}
	// The absolute gap across the spring-forward boundary (Mar 7 -> Mar 8) is
	// 23 hours, not 24: the wall clock held but an hour was skipped.
	loc, _ := time.LoadLocation("America/New_York")
	d0 := localToTime(*occ[0].Start, loc)
	d1 := localToTime(*occ[1].Start, loc)
	if got := d1.Sub(d0); got != 23*time.Hour {
		t.Errorf("gap across spring-forward = %v, want 23h", got)
	}
}

func TestTaskOccurrencesDaily(t *testing.T) {
	start := mustLocal(t, "2020-01-01T08:00:00")
	due := mustLocal(t, "2020-01-01T17:00:00")
	tk := &jscalendar.Task{
		Type: "Task", UID: "u", Start: &start, Due: &due, TimeZone: "UTC",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Count: uintPtr(2)},
		},
	}
	occ, err := TaskOccurrences(tk, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(occ) != 2 {
		t.Fatalf("got %d task occurrences, want 2", len(occ))
	}
	if occ[1].Start.String() != "2020-01-02T08:00:00" {
		t.Errorf("second task start = %s, want 2020-01-02T08:00:00", occ[1].Start)
	}
	for _, o := range occ {
		if o.RecurrenceID == nil || o.RecurrenceIDTimeZone != "UTC" {
			t.Errorf("task occurrence missing recurrenceId coupling")
		}
		if err := o.Validate(); err != nil {
			t.Errorf("task occurrence does not validate: %v", err)
		}
	}
}

func TestOccurrencesCustomFixedZone(t *testing.T) {
	start := mustLocal(t, "2020-06-01T12:00:00")
	e := &jscalendar.Event{
		Type: "Event", UID: "u", Start: &start, TimeZone: "/custom",
		TimeZones: map[jscalendar.TimeZoneId]jscalendar.TimeZone{
			"/custom": {
				Type: "TimeZone", TzID: "/custom",
				Standard: []jscalendar.TimeZoneRule{
					{Type: "TimeZoneRule", OffsetFrom: "+05:00", OffsetTo: "+05:00"},
				},
			},
		},
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyDaily, Count: uintPtr(2)},
		},
	}
	occ, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
		local(t, "UTC", "2021-01-01T00:00:00"))
	if err != nil {
		t.Fatalf("custom fixed zone should resolve: %v", err)
	}
	if len(occ) != 2 {
		t.Fatalf("got %d occurrences, want 2", len(occ))
	}
}

func TestOccurrencesErrors(t *testing.T) {
	t.Run("nil event", func(t *testing.T) {
		if _, err := Occurrences(nil, time.Time{}, time.Time{}); err == nil {
			t.Error("expected error for nil event")
		}
	})
	t.Run("no start", func(t *testing.T) {
		e := &jscalendar.Event{Type: "Event", UID: "u"}
		if _, err := Occurrences(e, time.Time{}, time.Time{}); err == nil {
			t.Error("expected error for event with no start")
		}
	})
	t.Run("unsupported rscale", func(t *testing.T) {
		start := mustLocal(t, "2020-01-01T00:00:00")
		e := &jscalendar.Event{
			Type: "Event", UID: "u", Start: &start, TimeZone: "UTC",
			RecurrenceRules: []jscalendar.RecurrenceRule{
				{Frequency: jscalendar.FrequencyYearly, RScale: "chinese"},
			},
		}
		_, err := Occurrences(e, local(t, "UTC", "2020-01-01T00:00:00"),
			local(t, "UTC", "2021-01-01T00:00:00"))
		if err == nil {
			t.Error("expected error for non-Gregorian rscale")
		}
	})
	t.Run("undefined custom zone", func(t *testing.T) {
		start := mustLocal(t, "2020-01-01T00:00:00")
		e := &jscalendar.Event{Type: "Event", UID: "u", Start: &start, TimeZone: "/missing"}
		if _, err := Occurrences(e, time.Time{}, time.Time{}.AddDate(1, 0, 0)); err == nil {
			t.Error("expected error for undefined custom zone")
		}
	})
}
