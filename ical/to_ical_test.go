// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical_test

import (
	"bytes"
	"encoding/json"
	"testing"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/ical"
)

// jsonRaw is the raw-JSON element type of a Group's entries, aliased for
// readable fixtures.
type jsonRaw = json.RawMessage

// encodeICS encodes a calendar to its iCalendar text, the gate that the
// produced components satisfy go-ical's required-property checks.
func encodeICS(cal *goical.Calendar) (string, error) {
	var buf bytes.Buffer
	if err := goical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// toCal converts objects with ToICal, failing the test on error, and returns
// the resulting calendar.
func toCal(t *testing.T, objs ...any) *goical.Calendar {
	t.Helper()
	cal, err := ical.ToICal(objs...)
	if err != nil {
		t.Fatalf("ToICal: %v", err)
	}
	return cal
}

// firstComponent returns the first child component of cal with the given name,
// failing the test when none is present.
func firstComponent(t *testing.T, cal *goical.Calendar, name string) *goical.Component {
	t.Helper()
	for _, child := range cal.Children {
		if child.Name == name {
			return child
		}
	}
	t.Fatalf("no %s component in calendar", name)
	return nil
}

// ptr returns a pointer to v, for building optional fields in test fixtures.
func ptr[T any](v T) *T { return &v }

// TestToICalCalendarSkeleton confirms the produced calendar carries the
// mandatory VERSION and PRODID the encoder requires.
func TestToICalCalendarSkeleton(t *testing.T) {
	t.Parallel()

	cal := toCal(t, &jscalendar.Event{Type: "Event", UID: "e@example.com"})

	if got := cal.Props.Get(goical.PropVersion); got == nil || got.Value != "2.0" {
		t.Errorf("VERSION = %v, want 2.0", got)
	}
	if got := cal.Props.Get(goical.PropProductID); got == nil || got.Value == "" {
		t.Errorf("PRODID = %v, want non-empty", got)
	}
}

// TestEventToVEvent maps an Event onto a VEVENT with the mandatory UID and
// DTSTAMP and the common descriptive properties.
func TestEventToVEvent(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type:        "Event",
		UID:         "e@example.com",
		Title:       "Weekly sync",
		Description: "Team meeting",
		Sequence:    3,
		Privacy:     "private",
		Status:      "confirmed",
		Updated:     &jscalendar.UTCDateTime{Year: 2026, Month: 1, Day: 2, Hour: 9, Minute: 30},
		Start:       &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
		TimeZone:    "America/New_York",
		Duration:    ptr(jscalendar.Duration{Hours: 1}),
	}

	vevent := firstComponent(t, toCal(t, event), goical.CompEvent)

	if got := vevent.Props.Get(goical.PropUID); got == nil || got.Value != "e@example.com" {
		t.Errorf("UID = %v", got)
	}
	if got := vevent.Props.Get(goical.PropDateTimeStamp); got == nil {
		t.Error("DTSTAMP missing")
	}
	if got, _ := vevent.Props.Text(goical.PropSummary); got != "Weekly sync" {
		t.Errorf("SUMMARY = %q", got)
	}
	if got := vevent.Props.Get(goical.PropSequence); got == nil || got.Value != "3" {
		t.Errorf("SEQUENCE = %v, want plain 3 (no VALUE=TEXT param)", got)
	}
	if got := vevent.Props.Get(goical.PropClass); got == nil || got.Value != "PRIVATE" {
		t.Errorf("CLASS = %v, want PRIVATE", got)
	}
	if got := vevent.Props.Get(goical.PropStatus); got == nil || got.Value != "CONFIRMED" {
		t.Errorf("STATUS = %v, want CONFIRMED", got)
	}
}

// TestDTStartZonedRecompose is the load-bearing inverse case: a zoned start
// recomposes into DTSTART;TZID= with a bare DATE-TIME value (no Z, no folding).
func TestDTStartZonedRecompose(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start:    &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
		TimeZone: "America/New_York",
	}
	dtstart := firstComponent(t, toCal(t, event), goical.CompEvent).Props.Get(goical.PropDateTimeStart)

	if dtstart.Params.Get(goical.ParamTimezoneID) != "America/New_York" {
		t.Errorf("TZID = %q, want America/New_York", dtstart.Params.Get(goical.ParamTimezoneID))
	}
	if dtstart.Value != "20260115T090000" {
		t.Errorf("DTSTART = %q, want 20260115T090000 (no Z)", dtstart.Value)
	}
}

// TestDTStartUTCRecompose maps the Etc/UTC zone back to a Z-suffixed DATE-TIME.
func TestDTStartUTCRecompose(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start:    &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 14},
		TimeZone: "Etc/UTC",
	}
	dtstart := firstComponent(t, toCal(t, event), goical.CompEvent).Props.Get(goical.PropDateTimeStart)

	if dtstart.Value != "20260115T140000Z" {
		t.Errorf("DTSTART = %q, want 20260115T140000Z", dtstart.Value)
	}
	if dtstart.Params.Get(goical.ParamTimezoneID) != "" {
		t.Errorf("TZID = %q, want none for a UTC value", dtstart.Params.Get(goical.ParamTimezoneID))
	}
}

// TestDTStartAllDayRecompose maps a showWithoutTime start to a VALUE=DATE value.
func TestDTStartAllDayRecompose(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start:           &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15},
		ShowWithoutTime: true,
	}
	dtstart := firstComponent(t, toCal(t, event), goical.CompEvent).Props.Get(goical.PropDateTimeStart)

	if dtstart.ValueType() != goical.ValueDate {
		t.Errorf("VALUE = %q, want DATE", dtstart.ValueType())
	}
	if dtstart.Value != "20260115" {
		t.Errorf("DTSTART = %q, want 20260115", dtstart.Value)
	}
}

// TestDTStartFloatingRecompose maps a zone-less, non-all-day start to a bare
// DATE-TIME with neither a Z nor a TZID.
func TestDTStartFloatingRecompose(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start: &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
	}
	dtstart := firstComponent(t, toCal(t, event), goical.CompEvent).Props.Get(goical.PropDateTimeStart)

	if dtstart.Value != "20260115T090000" {
		t.Errorf("DTSTART = %q, want bare 20260115T090000", dtstart.Value)
	}
	if dtstart.Params.Get(goical.ParamTimezoneID) != "" || dtstart.ValueType() == goical.ValueDate {
		t.Error("floating DTSTART carried a zone or DATE type")
	}
}

// TestRRuleString is the recurrence centerpiece: a structured RecurrenceRule
// renders back into an RRULE string with FREQ first and the BY* parts in order.
func TestRRuleString(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start: &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 5, Hour: 9},
		RecurrenceRules: []jscalendar.RecurrenceRule{{
			Frequency:  jscalendar.FrequencyMonthly,
			Interval:   2,
			ByDay:      []jscalendar.NDay{{Day: "su", NthOfPeriod: ptr(-1)}},
			ByMonth:    []string{"3", "11"},
			ByMonthDay: []int{1, -1},
			Count:      ptr[uint](5),
		}},
	}
	rrule := firstComponent(t, toCal(t, event), goical.CompEvent).Props.Get(goical.PropRecurrenceRule)

	want := "FREQ=MONTHLY;INTERVAL=2;BYDAY=-1SU;BYMONTHDAY=1,-1;BYMONTH=3,11;COUNT=5"
	if rrule == nil || rrule.Value != want {
		t.Errorf("RRULE = %q,\n want %q", valueOf(rrule), want)
	}
}

// TestRRuleNoFrequency rejects a rule with no frequency, mirroring the strict
// marshal boundary.
func TestRRuleNoFrequency(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		RecurrenceRules: []jscalendar.RecurrenceRule{{Interval: 2}},
	}
	if _, err := ical.ToICal(event); err == nil {
		t.Fatal("ToICal accepted a frequency-less rule, want an error")
	}
}

// TestExRuleToExrule maps an excludedRecurrenceRules entry back to EXRULE.
func TestExRuleToExrule(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start:                   &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 5},
		RecurrenceRules:         []jscalendar.RecurrenceRule{{Frequency: jscalendar.FrequencyWeekly}},
		ExcludedRecurrenceRules: []jscalendar.RecurrenceRule{{Frequency: jscalendar.FrequencyWeekly, ByDay: []jscalendar.NDay{{Day: "sa"}, {Day: "su"}}}},
	}
	vevent := firstComponent(t, toCal(t, event), goical.CompEvent)

	exrule := vevent.Props.Get("EXRULE")
	if exrule == nil || exrule.Value != "FREQ=WEEKLY;BYDAY=SA,SU" {
		t.Errorf("EXRULE = %q, want FREQ=WEEKLY;BYDAY=SA,SU", valueOf(exrule))
	}
}

// TestValarmOffset maps an Alert with an OffsetTrigger to a VALARM with a
// DURATION TRIGGER.
func TestValarmOffset(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Alerts: map[jscalendar.Id]jscalendar.Alert{
			"1": {
				Action: "display",
				Trigger: jscalendar.NewTrigger(jscalendar.OffsetTrigger{
					Offset: jscalendar.SignedDuration{Negative: true, Duration: jscalendar.Duration{Minutes: 15}},
				}),
			},
		},
	}
	vevent := firstComponent(t, toCal(t, event), goical.CompEvent)
	if len(vevent.Children) != 1 || vevent.Children[0].Name != goical.CompAlarm {
		t.Fatalf("want one VALARM child, got %v", vevent.Children)
	}
	valarm := vevent.Children[0]
	if got := valarm.Props.Get(goical.PropAction); got == nil || got.Value != "DISPLAY" {
		t.Errorf("ACTION = %v, want DISPLAY", got)
	}
	if got := valarm.Props.Get(goical.PropTrigger); got == nil || got.Value != "-PT15M" {
		t.Errorf("TRIGGER = %v, want -PT15M", got)
	}
}

// TestValarmRelatedEndToICal maps relativeTo "end" back to RELATED=END.
func TestValarmRelatedEndToICal(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Alerts: map[jscalendar.Id]jscalendar.Alert{
			"1": {Action: "display", Trigger: jscalendar.NewTrigger(jscalendar.OffsetTrigger{
				Offset: jscalendar.SignedDuration{Duration: jscalendar.Duration{Minutes: 10}}, RelativeTo: "end",
			})},
		},
	}
	trigger := firstComponent(t, toCal(t, event), goical.CompEvent).Children[0].Props.Get(goical.PropTrigger)
	if trigger.Params.Get(goical.ParamRelated) != "END" {
		t.Errorf("RELATED = %q, want END", trigger.Params.Get(goical.ParamRelated))
	}
}

// TestValarmAbsolute maps an AbsoluteTrigger to a DATE-TIME TRIGGER.
func TestValarmAbsolute(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Alerts: map[jscalendar.Id]jscalendar.Alert{
			"1": {Action: "email", Trigger: jscalendar.NewTrigger(jscalendar.AbsoluteTrigger{
				When: jscalendar.UTCDateTime{Year: 2026, Month: 1, Day: 5, Hour: 8, Minute: 30},
			})},
		},
	}
	trigger := firstComponent(t, toCal(t, event), goical.CompEvent).Children[0].Props.Get(goical.PropTrigger)
	if trigger.ValueType() != goical.ValueDateTime {
		t.Errorf("TRIGGER VALUE = %q, want DATE-TIME", trigger.ValueType())
	}
	if trigger.Value != "20260105T083000Z" {
		t.Errorf("TRIGGER = %q, want 20260105T083000Z", trigger.Value)
	}
}

// TestTaskToVTodo maps a Task onto a VTODO with its task-specific properties.
func TestTaskToVTodo(t *testing.T) {
	t.Parallel()

	task := &jscalendar.Task{
		Type: "Task", UID: "t@example.com", Title: "File taxes",
		Due:             &jscalendar.LocalDateTime{Year: 2026, Month: 4, Day: 15, Hour: 17},
		TimeZone:        "America/New_York",
		PercentComplete: ptr[uint](40),
		Progress:        "in-process",
	}
	vtodo := firstComponent(t, toCal(t, task), goical.CompToDo)

	due := vtodo.Props.Get(goical.PropDue)
	if due == nil || due.Value != "20260415T170000" || due.Params.Get(goical.ParamTimezoneID) != "America/New_York" {
		t.Errorf("DUE = %v", due)
	}
	if got := vtodo.Props.Get(goical.PropPercentComplete); got == nil || got.Value != "40" {
		t.Errorf("PERCENT-COMPLETE = %v, want plain 40", got)
	}
	if got := vtodo.Props.Get(goical.PropStatus); got == nil || got.Value != "IN-PROCESS" {
		t.Errorf("STATUS = %v, want IN-PROCESS", got)
	}
}

// TestVTimezoneEmitted maps an embedded custom timeZone back to a VTIMEZONE that
// precedes the referencing component, with the bare TZID restored on DTSTART.
func TestVTimezoneEmitted(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Start:    &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
		TimeZone: "/Custom/Local",
		TimeZones: map[jscalendar.TimeZoneId]jscalendar.TimeZone{
			"/Custom/Local": {
				TzID: "Custom/Local",
				Standard: []jscalendar.TimeZoneRule{{
					Start:      &jscalendar.LocalDateTime{Year: 2025, Month: 11, Day: 1, Hour: 2},
					OffsetFrom: "-04:00", OffsetTo: "-05:00",
					Names:           map[string]bool{"LST": true},
					RecurrenceRules: []jscalendar.RecurrenceRule{{Frequency: jscalendar.FrequencyYearly, ByMonth: []string{"11"}, ByDay: []jscalendar.NDay{{Day: "su", NthOfPeriod: ptr(1)}}}},
				}},
			},
		},
	}
	cal := toCal(t, event)

	// The VTIMEZONE must come before the VEVENT that references it.
	if len(cal.Children) < 2 || cal.Children[0].Name != goical.CompTimezone {
		t.Fatalf("first child = %v, want VTIMEZONE", cal.Children[0])
	}
	vtz := cal.Children[0]
	if got := vtz.Props.Get(goical.PropTimezoneID); got == nil || got.Value != "Custom/Local" {
		t.Errorf("TZID = %v, want Custom/Local", got)
	}
	std := vtz.Children[0]
	if got := std.Props.Get(goical.PropTimezoneOffsetTo); got == nil || got.Value != "-0500" {
		t.Errorf("TZOFFSETTO = %v, want -0500 (colon stripped)", got)
	}
	if got := std.Props.Get(goical.PropRecurrenceRule); got == nil || got.Value != "FREQ=YEARLY;BYDAY=1SU;BYMONTH=11" {
		t.Errorf("VTIMEZONE RRULE = %v", valueOf(got))
	}

	// The DTSTART on the event must use the bare TZID, not the /-prefixed id.
	dtstart := firstComponent(t, cal, goical.CompEvent).Props.Get(goical.PropDateTimeStart)
	if dtstart.Params.Get(goical.ParamTimezoneID) != "Custom/Local" {
		t.Errorf("DTSTART TZID = %q, want bare Custom/Local", dtstart.Params.Get(goical.ParamTimezoneID))
	}
}

// TestParticipantsToProps maps an owner participant to ORGANIZER and an attendee
// to ATTENDEE.
func TestParticipantsToProps(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e",
		Participants: map[jscalendar.Id]jscalendar.Participant{
			"1": {Name: "Alice", Email: "alice@example.com", Roles: map[string]bool{"owner": true}, SendTo: map[string]string{"imip": "mailto:alice@example.com"}},
			"2": {Name: "Bob", Email: "bob@example.com", Roles: map[string]bool{"attendee": true}, SendTo: map[string]string{"imip": "mailto:bob@example.com"}},
		},
	}
	vevent := firstComponent(t, toCal(t, event), goical.CompEvent)

	org := vevent.Props.Get(goical.PropOrganizer)
	if org == nil || org.Value != "mailto:alice@example.com" || org.Params.Get(goical.ParamCommonName) != "Alice" {
		t.Errorf("ORGANIZER = %v", org)
	}
	att := vevent.Props.Get(goical.PropAttendee)
	if att == nil || att.Value != "mailto:bob@example.com" {
		t.Errorf("ATTENDEE = %v", att)
	}
}

// TestGroupExpands converts a Group into the components of its entries.
func TestGroupExpands(t *testing.T) {
	t.Parallel()

	group := &jscalendar.Group{
		Type: "Group", UID: "g@example.com",
		Entries: []jsonRaw{
			[]byte(`{"@type":"Event","uid":"e1","start":"2026-01-15T09:00:00","timeZone":"Etc/UTC"}`),
			[]byte(`{"@type":"Task","uid":"t1"}`),
		},
	}
	cal := toCal(t, group)

	var events, todos int
	for _, child := range cal.Children {
		switch child.Name {
		case goical.CompEvent:
			events++
		case goical.CompToDo:
			todos++
		}
	}
	if events != 1 || todos != 1 {
		t.Errorf("got %d VEVENT and %d VTODO, want one each", events, todos)
	}
}

// TestToICalEncodes confirms a produced calendar encodes through go-ical's
// encoder, the property/required-prop validation gate.
func TestToICalEncodes(t *testing.T) {
	t.Parallel()

	event := &jscalendar.Event{
		Type: "Event", UID: "e@example.com",
		Start:    &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
		TimeZone: "Etc/UTC",
		Duration: ptr(jscalendar.Duration{Hours: 1}),
	}
	cal := toCal(t, event)
	if _, err := encodeICS(cal); err != nil {
		t.Fatalf("encode produced calendar: %v", err)
	}
}

// TestToICalUnsupportedWraps confirms an unsupported argument type produces an
// error (the package keeps the sentinel unexported, so this only asserts the
// failure, not its identity).
func TestToICalUnsupportedWraps(t *testing.T) {
	t.Parallel()

	if _, err := ical.ToICal(42); err == nil {
		t.Fatal("ToICal accepted an int, want an error")
	}
	// A nil interface argument is rejected rather than panicking.
	if _, err := ical.ToICal(any(nil)); err == nil {
		t.Fatal("ToICal accepted a nil object, want an error")
	}
	// A typed-nil pointer (which the type switch routes to the concrete case)
	// is rejected before any field is dereferenced, not panicked on.
	if _, err := ical.ToICal((*jscalendar.Event)(nil)); err == nil {
		t.Fatal("ToICal accepted a typed-nil *Event, want an error")
	}
}

// TestToICalRejectsControlChars confirms a value carrying a CR/LF is rejected
// rather than emitted, closing the iCalendar content-line injection vector. The
// CN parameter is the load-bearing case: go-ical's encoder rejects a CR/LF in a
// property value but not in a parameter value, so ToICal must reject it itself.
func TestToICalRejectsControlChars(t *testing.T) {
	t.Parallel()

	cases := map[string]*jscalendar.Event{
		"participant name (CN parameter)": {
			Type: "Event", UID: "e",
			Participants: map[jscalendar.Id]jscalendar.Participant{
				"1": {
					Name:   "Bob\r\nSUMMARY:Injected",
					Roles:  map[string]bool{"attendee": true},
					SendTo: map[string]string{"imip": "mailto:bob@example.com"},
				},
			},
		},
		"title (property value)": {
			Type: "Event", UID: "e",
			Title: "Hello\r\nDTSTART:20000101T000000Z",
		},
		"custom timezone id (TZID parameter)": {
			Type: "Event", UID: "e",
			Start:    &jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9},
			TimeZone: "/Bad\r\nX:Y",
			TimeZones: map[jscalendar.TimeZoneId]jscalendar.TimeZone{
				"/Bad\r\nX:Y": {TzID: "Bad\r\nX:Y"},
			},
		},
	}

	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ical.ToICal(event); err == nil {
				t.Fatalf("ToICal accepted a value with an embedded CRLF (%s), want rejection", name)
			}
		})
	}
}

// valueOf returns a prop's value or "<nil>" for nil, for readable failures.
func valueOf(p *goical.Prop) string {
	if p == nil {
		return "<nil>"
	}
	return p.Value
}
