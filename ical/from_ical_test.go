// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/ical"
)

// decodeICS parses iCalendar text into a *goical.Calendar, failing the test on
// a decode error.
func decodeICS(t *testing.T, text string) *goical.Calendar {
	t.Helper()
	cal, err := goical.NewDecoder(strings.NewReader(text)).Decode()
	if err != nil {
		t.Fatalf("decode ics: %v", err)
	}
	return cal
}

// fromICS parses iCalendar text and converts it, failing the test on error.
func fromICS(t *testing.T, text string) []any {
	t.Helper()
	objs, err := ical.FromICal(decodeICS(t, text))
	if err != nil {
		t.Fatalf("FromICal: %v", err)
	}
	return objs
}

// singleEvent converts text expected to hold exactly one VEVENT and returns the
// resulting Event.
func singleEvent(t *testing.T, text string) *jscalendar.Event {
	t.Helper()
	objs := fromICS(t, text)
	if len(objs) != 1 {
		t.Fatalf("got %d objects, want 1", len(objs))
	}
	event, ok := objs[0].(*jscalendar.Event)
	if !ok {
		t.Fatalf("got %T, want *jscalendar.Event", objs[0])
	}
	return event
}

// wrapVEvent wraps VEVENT body lines in a minimal VCALENDAR with the mandatory
// UID and DTSTAMP, so each test states only the properties it exercises.
func wrapVEvent(body string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:test@example.com\r\nDTSTAMP:20260101T000000Z\r\n" +
		body + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

// TestDTStartZonedSplit is the load-bearing case: DTSTART;TZID= must split into
// a zone-free start LocalDateTime and a separate timeZone, never folding the
// zone into the wall clock.
func TestDTStartZonedSplit(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent("DTSTART;TZID=America/New_York:20260115T090000"))

	if event.Start == nil {
		t.Fatal("Start is nil")
	}
	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9}
	if *event.Start != want {
		t.Errorf("Start = %+v, want %+v", *event.Start, want)
	}
	if event.TimeZone != "America/New_York" {
		t.Errorf("TimeZone = %q, want America/New_York", event.TimeZone)
	}
	if event.ShowWithoutTime {
		t.Error("ShowWithoutTime = true, want false for a zoned timed event")
	}
}

// TestDTStartUTCSplit maps a Z-suffixed DTSTART to the wall clock plus Etc/UTC.
func TestDTStartUTCSplit(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent("DTSTART:20260115T140000Z"))

	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 14}
	if event.Start == nil || *event.Start != want {
		t.Errorf("Start = %v, want %+v", event.Start, want)
	}
	if event.TimeZone != "Etc/UTC" {
		t.Errorf("TimeZone = %q, want Etc/UTC", event.TimeZone)
	}
}

// TestDTStartFloating maps a bare DATE-TIME (no Z, no TZID) to a floating
// LocalDateTime with no timeZone.
func TestDTStartFloating(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent("DTSTART:20260115T090000"))

	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15, Hour: 9}
	if event.Start == nil || *event.Start != want {
		t.Errorf("Start = %v, want %+v", event.Start, want)
	}
	if event.TimeZone != "" {
		t.Errorf("TimeZone = %q, want empty (floating)", event.TimeZone)
	}
}

// TestDTStartAllDay maps a DATE-valued DTSTART to a midnight start with
// showWithoutTime set and no zone.
func TestDTStartAllDay(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent("DTSTART;VALUE=DATE:20260115"))

	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 15}
	if event.Start == nil || *event.Start != want {
		t.Errorf("Start = %v, want %+v", event.Start, want)
	}
	if !event.ShowWithoutTime {
		t.Error("ShowWithoutTime = false, want true for an all-day event")
	}
	if event.TimeZone != "" {
		t.Errorf("TimeZone = %q, want empty for all-day", event.TimeZone)
	}
}

// TestDurationFromDTEnd derives Duration from a DTEND a fixed span after
// DTSTART, both in the same zone.
func TestDurationFromDTEnd(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART;TZID=America/New_York:20260115T090000\r\n"+
			"DTEND;TZID=America/New_York:20260115T103000",
	))

	if event.Duration == nil {
		t.Fatal("Duration is nil")
	}
	if got := event.Duration.String(); got != "PT1H30M" {
		t.Errorf("Duration = %q, want PT1H30M", got)
	}
}

// TestDurationExplicit takes an explicit DURATION property verbatim.
func TestDurationExplicit(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260115T090000Z\r\nDURATION:PT45M",
	))

	if event.Duration == nil || event.Duration.String() != "PT45M" {
		t.Errorf("Duration = %v, want PT45M", event.Duration)
	}
}

// TestDurationAllDay derives a whole-day duration from an all-day DTSTART/DTEND
// pair.
func TestDurationAllDay(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART;VALUE=DATE:20260115\r\nDTEND;VALUE=DATE:20260118",
	))

	if event.Duration == nil || event.Duration.String() != "P3D" {
		t.Errorf("Duration = %v, want P3D", event.Duration)
	}
}

// TestRRuleStructured is the recurrence centerpiece: the opaque RRULE string
// must decompose into the structured RecurrenceRule fields.
func TestRRuleStructured(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"RRULE:FREQ=MONTHLY;INTERVAL=2;BYDAY=-1SU;BYMONTH=3,11;BYMONTHDAY=1,-1;COUNT=5",
	))

	if len(event.RecurrenceRules) != 1 {
		t.Fatalf("got %d rules, want 1", len(event.RecurrenceRules))
	}
	rule := event.RecurrenceRules[0]

	if rule.Frequency != jscalendar.FrequencyMonthly {
		t.Errorf("Frequency = %q, want monthly", rule.Frequency)
	}
	if rule.Interval != 2 {
		t.Errorf("Interval = %d, want 2", rule.Interval)
	}
	if len(rule.ByDay) != 1 || rule.ByDay[0].Day != "su" || rule.ByDay[0].NthOfPeriod == nil || *rule.ByDay[0].NthOfPeriod != -1 {
		t.Errorf("ByDay = %+v, want [{su -1}]", rule.ByDay)
	}
	if len(rule.ByMonth) != 2 || rule.ByMonth[0] != "3" || rule.ByMonth[1] != "11" {
		t.Errorf("ByMonth = %v, want [3 11] (strings)", rule.ByMonth)
	}
	if len(rule.ByMonthDay) != 2 || rule.ByMonthDay[0] != 1 || rule.ByMonthDay[1] != -1 {
		t.Errorf("ByMonthDay = %v, want [1 -1]", rule.ByMonthDay)
	}
	if !rule.HasCount() || *rule.Count != 5 {
		t.Errorf("Count = %v, want 5", rule.Count)
	}
	if rule.HasUntil() {
		t.Error("HasUntil = true, want false (COUNT and UNTIL are exclusive)")
	}
}

// TestRRuleByParts exercises the remaining BY* parts, which route through
// distinct signed-int vs. unsigned-int parsers (BYSETPOS and BYWEEKNO/BYYEARDAY
// admit negatives; BYHOUR/BYMINUTE/BYSECOND do not) plus WKST.
func TestRRuleByParts(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"RRULE:FREQ=YEARLY;WKST=SU;BYHOUR=9,17;BYMINUTE=30;BYSECOND=0;"+
			"BYYEARDAY=1,-1;BYWEEKNO=1,-1;BYSETPOS=-1",
	))

	rule := event.RecurrenceRules[0]
	if rule.FirstDayOfWeek != "su" {
		t.Errorf("FirstDayOfWeek = %q, want su", rule.FirstDayOfWeek)
	}
	if len(rule.ByHour) != 2 || rule.ByHour[0] != 9 || rule.ByHour[1] != 17 {
		t.Errorf("ByHour = %v, want [9 17]", rule.ByHour)
	}
	if len(rule.ByMinute) != 1 || rule.ByMinute[0] != 30 {
		t.Errorf("ByMinute = %v, want [30]", rule.ByMinute)
	}
	if len(rule.BySecond) != 1 || rule.BySecond[0] != 0 {
		t.Errorf("BySecond = %v, want [0]", rule.BySecond)
	}
	if len(rule.ByYearDay) != 2 || rule.ByYearDay[1] != -1 {
		t.Errorf("ByYearDay = %v, want [1 -1]", rule.ByYearDay)
	}
	if len(rule.ByWeekNo) != 2 || rule.ByWeekNo[1] != -1 {
		t.Errorf("ByWeekNo = %v, want [1 -1]", rule.ByWeekNo)
	}
	if len(rule.BySetPosition) != 1 || rule.BySetPosition[0] != -1 {
		t.Errorf("BySetPosition = %v, want [-1]", rule.BySetPosition)
	}
}

// TestRRuleUntilDate confirms a DATE-form UNTIL (no time component) becomes a
// midnight LocalDateTime, exercising the date branch of parseUntil.
func TestRRuleUntilDate(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\nRRULE:FREQ=WEEKLY;UNTIL=20260131",
	))

	rule := event.RecurrenceRules[0]
	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 31}
	if rule.Until == nil || *rule.Until != want {
		t.Errorf("Until = %v, want %+v", rule.Until, want)
	}
}

// TestRRuleUntilLocal confirms an UNTIL with a trailing Z is carried as a
// zone-free LocalDateTime: JSCalendar interprets until in the object's own zone,
// so the Z designator is dropped rather than folded into a UTC instant.
func TestRRuleUntilLocal(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART;TZID=America/New_York:20260105T090000\r\n"+
			"RRULE:FREQ=DAILY;UNTIL=20260131T235959Z",
	))

	rule := event.RecurrenceRules[0]
	if rule.Until == nil {
		t.Fatal("Until is nil")
	}
	want := jscalendar.LocalDateTime{Year: 2026, Month: 1, Day: 31, Hour: 23, Minute: 59, Second: 59}
	if *rule.Until != want {
		t.Errorf("Until = %+v, want %+v", *rule.Until, want)
	}
}

// TestExRuleExcluded maps an EXRULE to an excludedRecurrenceRules entry.
func TestExRuleExcluded(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"RRULE:FREQ=WEEKLY\r\n"+
			"EXRULE:FREQ=WEEKLY;BYDAY=SA,SU",
	))

	if len(event.RecurrenceRules) != 1 {
		t.Fatalf("got %d recurrence rules, want 1", len(event.RecurrenceRules))
	}
	if len(event.ExcludedRecurrenceRules) != 1 {
		t.Fatalf("got %d excluded rules, want 1", len(event.ExcludedRecurrenceRules))
	}
	if len(event.ExcludedRecurrenceRules[0].ByDay) != 2 {
		t.Errorf("excluded ByDay = %+v, want 2 entries", event.ExcludedRecurrenceRules[0].ByDay)
	}
}

// TestValarmOffsetTrigger maps a VALARM with a DURATION trigger to an Alert
// carrying an OffsetTrigger.
func TestValarmOffsetTrigger(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nEND:VALARM",
	))

	if len(event.Alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(event.Alerts))
	}
	alert := event.Alerts["1"]
	if alert.Action != "display" {
		t.Errorf("Action = %q, want display", alert.Action)
	}
	offset, ok := alert.Trigger.Value().(jscalendar.OffsetTrigger)
	if !ok {
		t.Fatalf("Trigger.Value = %T, want OffsetTrigger", alert.Trigger.Value())
	}
	if !offset.Offset.Negative || offset.Offset.String() != "-PT15M" {
		t.Errorf("Offset = %q, want -PT15M", offset.Offset.String())
	}
}

// TestValarmAbsoluteTrigger maps a VALARM with a DATE-TIME trigger to an Alert
// carrying an AbsoluteTrigger.
func TestValarmAbsoluteTrigger(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"BEGIN:VALARM\r\nACTION:EMAIL\r\nTRIGGER;VALUE=DATE-TIME:20260105T083000Z\r\nEND:VALARM",
	))

	alert := event.Alerts["1"]
	if alert.Action != "email" {
		t.Errorf("Action = %q, want email", alert.Action)
	}
	abs, ok := alert.Trigger.Value().(jscalendar.AbsoluteTrigger)
	if !ok {
		t.Fatalf("Trigger.Value = %T, want AbsoluteTrigger", alert.Trigger.Value())
	}
	want := jscalendar.UTCDateTime{Year: 2026, Month: 1, Day: 5, Hour: 8, Minute: 30}
	if abs.When != want {
		t.Errorf("When = %+v, want %+v", abs.When, want)
	}
}

// TestValarmRelatedEnd maps RELATED=END to a relativeTo of "end".
func TestValarmRelatedEnd(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER;RELATED=END:PT10M\r\nEND:VALARM",
	))

	offset, ok := event.Alerts["1"].Trigger.Value().(jscalendar.OffsetTrigger)
	if !ok {
		t.Fatalf("Trigger.Value = %T, want OffsetTrigger", event.Alerts["1"].Trigger.Value())
	}
	if offset.RelativeTo != "end" {
		t.Errorf("RelativeTo = %q, want end", offset.RelativeTo)
	}
}

// TestCommonProps maps the metadata, descriptive, and sub-object properties
// shared by events and tasks.
func TestCommonProps(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent(
		"DTSTART:20260105T090000Z\r\n"+
			"SUMMARY:Quarterly review\r\n"+
			"DESCRIPTION:Q1 numbers\r\n"+
			"SEQUENCE:7\r\n"+
			"CREATED:20251201T080000Z\r\n"+
			"LAST-MODIFIED:20260102T093000Z\r\n"+
			"CLASS:CONFIDENTIAL\r\n"+
			"STATUS:TENTATIVE\r\n"+
			"LOCATION:Boardroom\r\n"+
			"URL:https://example.com/q1\r\n"+
			"CATEGORIES:finance,review\r\n"+
			"ORGANIZER;CN=Alice:mailto:alice@example.com\r\n"+
			"ATTENDEE;CN=Bob:mailto:bob@example.com",
	))

	if event.UID != "test@example.com" {
		t.Errorf("UID = %q", event.UID)
	}
	if event.Title != "Quarterly review" {
		t.Errorf("Title = %q", event.Title)
	}
	if event.Description != "Q1 numbers" {
		t.Errorf("Description = %q", event.Description)
	}
	if event.Sequence != 7 {
		t.Errorf("Sequence = %d, want 7", event.Sequence)
	}
	if event.Created == nil || event.Created.Year != 2025 {
		t.Errorf("Created = %v", event.Created)
	}
	if event.Updated == nil || event.Updated.Day != 2 {
		t.Errorf("Updated = %v, want LAST-MODIFIED 2026-01-02", event.Updated)
	}
	if event.Privacy != "secret" {
		t.Errorf("Privacy = %q, want secret (from CONFIDENTIAL)", event.Privacy)
	}
	if event.Status != "tentative" {
		t.Errorf("Status = %q, want tentative", event.Status)
	}
	if loc := event.Locations["1"]; loc.Name != "Boardroom" {
		t.Errorf("Location[1].Name = %q, want Boardroom", loc.Name)
	}
	if link := event.Links["1"]; link.Href != "https://example.com/q1" {
		t.Errorf("Links[1].Href = %q", link.Href)
	}
	if !event.Keywords["finance"] || !event.Keywords["review"] {
		t.Errorf("Keywords = %v, want finance+review", event.Keywords)
	}
	if owner := event.Participants["1"]; !owner.Roles["owner"] || owner.Email != "alice@example.com" {
		t.Errorf("Participants[1] = %+v, want owner alice", owner)
	}
	if att := event.Participants["2"]; !att.Roles["attendee"] || att.Name != "Bob" {
		t.Errorf("Participants[2] = %+v, want attendee Bob", att)
	}
}

// TestUpdatedFallsBackToDTStamp confirms updated uses DTSTAMP when no
// LAST-MODIFIED is present.
func TestUpdatedFallsBackToDTStamp(t *testing.T) {
	t.Parallel()

	event := singleEvent(t, wrapVEvent("DTSTART:20260105T090000Z"))
	if event.Updated == nil || event.Updated.Year != 2026 {
		t.Errorf("Updated = %v, want DTSTAMP fallback", event.Updated)
	}
}

// TestVTodoToTask maps a VTODO to a *jscalendar.Task with its task-specific
// properties.
func TestVTodoToTask(t *testing.T) {
	t.Parallel()

	const cal = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VTODO\r\nUID:todo@example.com\r\nDTSTAMP:20260101T000000Z\r\n" +
		"SUMMARY:File taxes\r\n" +
		"DUE;TZID=America/New_York:20260415T170000\r\n" +
		"PERCENT-COMPLETE:40\r\n" +
		"STATUS:IN-PROCESS\r\n" +
		"END:VTODO\r\nEND:VCALENDAR\r\n"

	objs := fromICS(t, cal)
	if len(objs) != 1 {
		t.Fatalf("got %d objects, want 1", len(objs))
	}
	task, ok := objs[0].(*jscalendar.Task)
	if !ok {
		t.Fatalf("got %T, want *jscalendar.Task", objs[0])
	}
	if task.Title != "File taxes" {
		t.Errorf("Title = %q", task.Title)
	}
	wantDue := jscalendar.LocalDateTime{Year: 2026, Month: 4, Day: 15, Hour: 17}
	if task.Due == nil || *task.Due != wantDue {
		t.Errorf("Due = %v, want %+v", task.Due, wantDue)
	}
	if task.TimeZone != "America/New_York" {
		t.Errorf("TimeZone = %q, want America/New_York", task.TimeZone)
	}
	if task.PercentComplete == nil || *task.PercentComplete != 40 {
		t.Errorf("PercentComplete = %v, want 40", task.PercentComplete)
	}
	if task.Progress != "in-process" {
		t.Errorf("Progress = %q, want in-process", task.Progress)
	}
}

// TestVTimezoneEmbedded maps a VTIMEZONE to an embedded timeZones entry under a
// "/"-prefixed custom id, and rewrites a referencing event's timeZone to that
// id so the reference resolves.
func TestVTimezoneEmbedded(t *testing.T) {
	t.Parallel()

	const cal = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:Custom/Local\r\n" +
		"BEGIN:STANDARD\r\nDTSTART:20251101T020000\r\n" +
		"TZOFFSETFROM:-0400\r\nTZOFFSETTO:-0500\r\nTZNAME:LST\r\n" +
		"RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU\r\nEND:STANDARD\r\n" +
		"BEGIN:DAYLIGHT\r\nDTSTART:20260308T020000\r\n" +
		"TZOFFSETFROM:-0500\r\nTZOFFSETTO:-0400\r\nTZNAME:LDT\r\n" +
		"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU\r\nEND:DAYLIGHT\r\n" +
		"END:VTIMEZONE\r\n" +
		"BEGIN:VEVENT\r\nUID:tz@example.com\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART;TZID=Custom/Local:20260115T090000\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	event := singleEvent(t, cal)

	if event.TimeZone != "/Custom/Local" {
		t.Errorf("TimeZone = %q, want /Custom/Local", event.TimeZone)
	}
	tz, ok := event.TimeZones["/Custom/Local"]
	if !ok {
		t.Fatalf("timeZones missing /Custom/Local; have %v", event.TimeZones)
	}
	if tz.TzID != "Custom/Local" {
		t.Errorf("tz.TzID = %q, want Custom/Local", tz.TzID)
	}
	if len(tz.Standard) != 1 || tz.Standard[0].OffsetTo != "-05:00" {
		t.Errorf("Standard = %+v, want one rule with offsetTo -05:00", tz.Standard)
	}
	if len(tz.Daylight) != 1 || tz.Daylight[0].OffsetFrom != "-05:00" {
		t.Errorf("Daylight = %+v, want one rule with offsetFrom -05:00", tz.Daylight)
	}
	if !tz.Standard[0].Names["LST"] {
		t.Errorf("Standard names = %v, want LST", tz.Standard[0].Names)
	}
	if len(tz.Daylight[0].RecurrenceRules) != 1 {
		t.Errorf("Daylight rule has no recurrence; want one from its RRULE")
	}
}

// TestMultipleComponents converts a calendar with both a VEVENT and a VTODO,
// returning the two concrete types in document order.
func TestMultipleComponents(t *testing.T) {
	t.Parallel()

	const cal = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:e@example.com\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART:20260105T090000Z\r\nEND:VEVENT\r\n" +
		"BEGIN:VTODO\r\nUID:t@example.com\r\nDTSTAMP:20260101T000000Z\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n"

	objs := fromICS(t, cal)
	if len(objs) != 2 {
		t.Fatalf("got %d objects, want 2", len(objs))
	}
	if _, ok := objs[0].(*jscalendar.Event); !ok {
		t.Errorf("objs[0] = %T, want *jscalendar.Event", objs[0])
	}
	if _, ok := objs[1].(*jscalendar.Task); !ok {
		t.Errorf("objs[1] = %T, want *jscalendar.Task", objs[1])
	}
}

// TestProducedObjectsValidate confirms every converted object passes the
// parent package's opt-in Validate, the acceptance criterion that FromICal is
// correct enough to support the round-trip conformance work.
func TestProducedObjectsValidate(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "event_zoned.ics"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	objs, err := ical.FromICal(decodeICS(t, string(data)))
	if err != nil {
		t.Fatalf("FromICal: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("no objects converted")
	}
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
		// The converted object must also marshal cleanly: the strict-marshal
		// boundary catches a structurally invalid value (e.g. a RecurrenceRule
		// with no frequency) that Validate's MUST-set does not. This is the
		// acceptance bar for supporting the round-trip corpus.
		if _, err := json.Marshal(obj); err != nil {
			t.Errorf("object %d marshal: %v", i, err)
		}
	}
}

// TestTestdataEventFields spot-checks the parsed testdata event end to end,
// exercising the .ics-text decode path in addition to the programmatic cases.
func TestTestdataEventFields(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "event_zoned.ics"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	event := singleEvent(t, string(data))

	if event.UID != "zoned-event@example.com" {
		t.Errorf("UID = %q", event.UID)
	}
	if event.TimeZone != "America/New_York" {
		t.Errorf("TimeZone = %q", event.TimeZone)
	}
	if event.Duration == nil || event.Duration.String() != "PT1H" {
		t.Errorf("Duration = %v, want PT1H", event.Duration)
	}
	if len(event.RecurrenceRules) != 1 || event.RecurrenceRules[0].Frequency != jscalendar.FrequencyWeekly {
		t.Errorf("RecurrenceRules = %+v", event.RecurrenceRules)
	}
	if len(event.Alerts) != 1 {
		t.Errorf("Alerts = %v, want 1", event.Alerts)
	}
}
