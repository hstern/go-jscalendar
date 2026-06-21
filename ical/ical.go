// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	goical "github.com/emersion/go-ical"
)

// ToICal converts one or more JSCalendar objects into a single iCalendar
// calendar.
//
// Each argument must be one of the concrete pointer types the parent package
// produces — a *jscalendar.Event, *jscalendar.Task, or *jscalendar.Group —
// typically obtained from jscalendar.Parse. Each Event becomes a VEVENT and
// each Task a VTODO within the returned [goical.Calendar]; a Group's entries
// expand into their respective components. An argument of any other dynamic
// type is a conversion error wrapping [errUnsupportedType] (testable with
// [errors.Is]).
//
// The returned calendar carries the mandatory VERSION and PRODID, and every
// component the mandatory UID and DTSTAMP, so it encodes through
// [goical.NewEncoder] without tripping the encoder's required-property checks.
// Any custom ("/"-prefixed) time zone an object references is emitted once as a
// VTIMEZONE ahead of the components, mirroring the layout [FromICal] consumes.
//
// A JSCalendar string value containing a control character (a CR, LF, or other
// C0 byte) is rejected rather than emitted: an embedded CR/LF would forge an
// iCalendar content line in the output. The error wraps an unexported sentinel
// and is distinct from the unsupported-type error.
//
// # Mapping
//
// ToICal is the inverse of [FromICal] and follows the same calext
// jscalendar-icalendar correspondence in reverse:
//
//   - start + timeZone recompose into DTSTART (";TZID=" for a named zone, a
//     trailing "Z" for "Etc/UTC", VALUE=DATE for an all-day showWithoutTime
//     value, a bare DATE-TIME for floating time); a Task's due maps to DUE.
//   - duration becomes a DURATION property (estimatedDuration for a Task).
//   - each structured [jscalendar.RecurrenceRule] becomes an RRULE string, and
//     each excludedRecurrenceRules entry becomes an EXRULE.
//   - each [jscalendar.Alert] becomes a VALARM (an offset or absolute trigger →
//     TRIGGER, action → ACTION).
//   - each embedded [jscalendar.TimeZone] becomes a VTIMEZONE with its STANDARD
//     / DAYLIGHT rules.
//   - the common properties: uid → UID, title → SUMMARY, description →
//     DESCRIPTION, sequence → SEQUENCE, created → CREATED, updated →
//     LAST-MODIFIED and DTSTAMP, privacy → CLASS, keywords → CATEGORIES,
//     locations → LOCATION, participants → ORGANIZER / ATTENDEE, links → URL,
//     status → STATUS. A Task additionally maps percentComplete →
//     PERCENT-COMPLETE and progress → STATUS.
//
// # Lossy edges
//
// Some JSCalendar constructs have no faithful iCalendar counterpart, so the
// round trip JSCalendar → iCalendar → JSCalendar is not exact for them. ToICal
// makes a deterministic, documented choice rather than failing:
//
//   - Multiple locations or links collapse to a single LOCATION / URL (the
//     lowest [jscalendar.Id] wins); the rest are dropped, since iCalendar
//     carries one of each.
//   - A participant role beyond "owner" / "attendee", and most participant
//     metadata (participationStatus, kind, the scheduling fields), have no
//     ORGANIZER / ATTENDEE form here and are not emitted.
//   - A DISPLAY VALARM is given a synthesized DESCRIPTION ("Reminder") to
//     satisfy RFC 5545, since a JSCalendar Alert has no per-alert description;
//     a round trip therefore introduces a description the source lacked.
//   - An alert trigger of a kind the model preserved as raw JSON has no TRIGGER
//     form and is dropped.
//   - Open-extension members (the parent package's Extra) and JSCalendar
//     properties with no iCalendar counterpart (e.g. color, virtualLocations,
//     relatedTo, localizations, recurrenceOverrides) are not emitted.
//
// The conformance corpus (conformance_interop_test.go) exercises the lossless
// edges as a semantic round trip and pins these lossy edges, so a future change
// that narrows them shows up as a visible test delta.
func ToICal(objs ...any) (*goical.Calendar, error) {
	return toICal(objs...)
}
