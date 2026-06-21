// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file implements FromICal: the iCalendar-to-JSCalendar direction of the
// conversion. The entry point walks a VCALENDAR's top-level components, mapping
// each VEVENT to a [jscalendar.Event] and each VTODO to a [jscalendar.Task],
// and shares the common-property mapping (UID, SUMMARY, DESCRIPTION, …) between
// them through the commonProps helper.
//
// The mapping follows the IETF calext working group's "JSCalendar: Converting
// from and to iCalendar" property correspondence. The doc comment on FromICal
// records the coverage boundary for v0.1.0.

// FromICal converts an iCalendar calendar into its JSCalendar objects.
//
// Each top-level VEVENT becomes a [*jscalendar.Event] and each top-level VTODO
// becomes a [*jscalendar.Task], returned in document order; the returned slice
// holds those concrete pointer types, matching the contract of
// jscalendar.Parse, so callers type-switch on each element. Components other
// than VEVENT and VTODO at the top level (a stray VTIMEZONE, a VJOURNAL) are
// skipped — a VTIMEZONE is consumed as the embedded timeZones of the events and
// tasks that reference it, not surfaced as its own object.
//
// # Mappings implemented
//
// The conversion covers the common calendar-object surface:
//
//   - DTSTART splits into start (a zone-free [jscalendar.LocalDateTime]) and
//     timeZone (a [jscalendar.TimeZoneId]). A ";TZID=" parameter becomes the
//     timeZone; a trailing "Z" maps to the "Etc/UTC" zone; a bare DATE-TIME is
//     floating (no timeZone); a DATE value sets showWithoutTime with no zone.
//   - DTEND / DURATION (and, for a Task, DUE / DURATION) derive the duration:
//     an explicit DURATION is parsed directly, a DTEND is differenced against
//     DTSTART.
//   - RRULE becomes a structured [jscalendar.RecurrenceRule]; an EXRULE, if
//     present, becomes an excludedRecurrenceRules entry.
//   - VALARM becomes an [jscalendar.Alert] (TRIGGER → OffsetTrigger /
//     AbsoluteTrigger, ACTION → action).
//   - VTIMEZONE becomes an embedded [jscalendar.TimeZone] in timeZones, keyed by
//     the "/"-prefixed TZID, with its STANDARD / DAYLIGHT rules.
//   - Common properties: UID, SUMMARY → title, DESCRIPTION, SEQUENCE, CREATED,
//     LAST-MODIFIED / DTSTAMP → updated, STATUS, CLASS → privacy, LOCATION → a
//     [jscalendar.Location], ORGANIZER / ATTENDEE → participants, URL → a
//     [jscalendar.Link], CATEGORIES → keywords. A Task additionally maps DUE,
//     PERCENT-COMPLETE, and STATUS → progress.
//
// # Coverage boundary (v0.1.0)
//
// Full iCalendar property coverage is out of scope for v0.1.0. VJOURNAL and
// VFREEBUSY are not mapped; EXDATE / RDATE individual-date overrides are not
// modeled (the recurrence-override map is the JSCalendar mechanism, and
// populating it from EXDATE is deferred); a VTIMEZONE's RDATE-based and
// metadata-rich forms are reduced to their RRULE-driven STANDARD / DAYLIGHT
// rules. Properties with no field in the JSCalendar model are not silently
// dropped into a typed field — they simply have no representation here and are
// left for a future release. The objects this produces validate cleanly under
// [jscalendar.Event.Validate] / [jscalendar.Task.Validate].
func FromICal(cal *goical.Calendar) ([]any, error) {
	if cal == nil || cal.Component == nil {
		return nil, nil
	}

	// Collect the calendar's VTIMEZONE definitions once: every event and task
	// that references a custom zone embeds the same timeZones map.
	timeZones, err := collectTimeZones(cal.Component)
	if err != nil {
		return nil, err
	}

	var objs []any
	for _, child := range cal.Children {
		switch child.Name {
		case goical.CompEvent:
			event, err := eventFromVEvent(child, timeZones)
			if err != nil {
				return nil, fmt.Errorf("ical: convert VEVENT: %w", err)
			}
			objs = append(objs, event)
		case goical.CompToDo:
			task, err := taskFromVTodo(child, timeZones)
			if err != nil {
				return nil, fmt.Errorf("ical: convert VTODO: %w", err)
			}
			objs = append(objs, task)
		default:
			// VTIMEZONE (consumed above), VJOURNAL, VFREEBUSY, and any other
			// top-level component are out of scope here; skip them.
		}
	}

	return objs, nil
}

// collectTimeZones walks the calendar's VTIMEZONE children into a map keyed by
// the custom, "/"-prefixed [jscalendar.TimeZoneId]. The map is embedded as the
// timeZones of any event or task whose timeZone references one of its keys.
func collectTimeZones(cal *goical.Component) (map[jscalendar.TimeZoneId]jscalendar.TimeZone, error) {
	var zones map[jscalendar.TimeZoneId]jscalendar.TimeZone
	for _, child := range cal.Children {
		if child.Name != goical.CompTimezone {
			continue
		}
		id, tz, err := timeZoneFromVTimezone(child)
		if err != nil {
			return nil, fmt.Errorf("ical: convert VTIMEZONE: %w", err)
		}
		if zones == nil {
			zones = map[jscalendar.TimeZoneId]jscalendar.TimeZone{}
		}
		zones[id] = tz
	}
	return zones, nil
}

// eventFromVEvent maps a VEVENT component onto a [*jscalendar.Event]: the common
// properties, then the event-specific start/duration split, status, and
// recurrence.
func eventFromVEvent(comp *goical.Component, timeZones map[jscalendar.TimeZoneId]jscalendar.TimeZone) (*jscalendar.Event, error) {
	event := &jscalendar.Event{Type: "Event"}
	if err := commonProps(comp, &commonTarget{
		uid:          &event.UID,
		title:        &event.Title,
		description:  &event.Description,
		sequence:     &event.Sequence,
		created:      &event.Created,
		updated:      &event.Updated,
		privacy:      &event.Privacy,
		keywords:     &event.Keywords,
		locations:    &event.Locations,
		participants: &event.Participants,
		links:        &event.Links,
		alerts:       &event.Alerts,
	}); err != nil {
		return nil, err
	}

	start, err := startProps(comp)
	if err != nil {
		return nil, err
	}
	if start != nil {
		event.Start = &start.local
		applyZone(start, &event.TimeZone, &event.ShowWithoutTime)
	}

	dur, err := eventDuration(comp, start)
	if err != nil {
		return nil, err
	}
	if dur != nil {
		event.Duration = dur
	}

	if status := rawValue(comp.Props, goical.PropStatus); status != "" {
		event.Status = strings.ToLower(status)
	}

	rules, excluded, err := recurrenceProps(comp)
	if err != nil {
		return nil, err
	}
	event.RecurrenceRules = rules
	event.ExcludedRecurrenceRules = excluded

	attachTimeZones(&event.TimeZone, timeZones, &event.TimeZones)

	return event, nil
}

// taskFromVTodo maps a VTODO component onto a [*jscalendar.Task]: the common
// properties, then the task-specific start, due, estimated duration, progress,
// and recurrence.
func taskFromVTodo(comp *goical.Component, timeZones map[jscalendar.TimeZoneId]jscalendar.TimeZone) (*jscalendar.Task, error) {
	task := &jscalendar.Task{Type: "Task"}
	if err := commonProps(comp, &commonTarget{
		uid:          &task.UID,
		title:        &task.Title,
		description:  &task.Description,
		sequence:     &task.Sequence,
		created:      &task.Created,
		updated:      &task.Updated,
		privacy:      &task.Privacy,
		keywords:     &task.Keywords,
		locations:    &task.Locations,
		participants: &task.Participants,
		links:        &task.Links,
		alerts:       &task.Alerts,
	}); err != nil {
		return nil, err
	}

	start, err := startProps(comp)
	if err != nil {
		return nil, err
	}
	if start != nil {
		task.Start = &start.local
		applyZone(start, &task.TimeZone, &task.ShowWithoutTime)
	}

	if dueProp := comp.Props.Get(goical.PropDue); dueProp != nil {
		due, err := splitDateTimeProp(dueProp)
		if err != nil {
			return nil, err
		}
		task.Due = &due.local
		// A Task carries a single timeZone; when there is no start to set it, the
		// due's zone governs.
		if task.TimeZone == "" {
			applyZone(&due, &task.TimeZone, &task.ShowWithoutTime)
		}
	}

	if durProp := rawValue(comp.Props, goical.PropDuration); durProp != "" {
		d, err := jscalendar.ParseDuration(durProp)
		if err != nil {
			return nil, err
		}
		task.EstimatedDuration = &d
	}

	if pct := rawValue(comp.Props, goical.PropPercentComplete); pct != "" {
		n, err := strconv.ParseUint(pct, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("ical: malformed PERCENT-COMPLETE %q: %w", pct, err)
		}
		v := uint(n)
		task.PercentComplete = &v
	}

	if status := rawValue(comp.Props, goical.PropStatus); status != "" {
		task.Progress = strings.ToLower(status)
	}

	rules, excluded, err := recurrenceProps(comp)
	if err != nil {
		return nil, err
	}
	task.RecurrenceRules = rules
	task.ExcludedRecurrenceRules = excluded

	attachTimeZones(&task.TimeZone, timeZones, &task.TimeZones)

	return task, nil
}

// startProps reads and splits a component's DTSTART, returning nil when the
// component has none (a VTODO may legitimately omit it).
func startProps(comp *goical.Component) (*splitDateTime, error) {
	startProp := comp.Props.Get(goical.PropDateTimeStart)
	if startProp == nil {
		return nil, nil
	}
	split, err := splitDateTimeProp(startProp)
	if err != nil {
		return nil, err
	}
	return &split, nil
}

// applyZone copies a split's zone onto the object's timeZone (when zoned) or
// sets showWithoutTime (when the source was an all-day DATE value). A floating
// value sets neither.
func applyZone(split *splitDateTime, timeZone *jscalendar.TimeZoneId, showWithoutTime *bool) {
	switch split.kind {
	case kindDate:
		*showWithoutTime = true
	case kindUTC, kindZoned:
		*timeZone = split.timeZone
	case kindLocal:
		// Floating: no zone, no showWithoutTime.
	}
}

// eventDuration derives an Event's duration from DTEND or DURATION relative to
// the (already split) DTSTART. It returns nil when neither is present.
func eventDuration(comp *goical.Component, start *splitDateTime) (*jscalendar.Duration, error) {
	if start == nil {
		return nil, nil
	}
	var end *splitDateTime
	if endProp := comp.Props.Get(goical.PropDateTimeEnd); endProp != nil {
		split, err := splitDateTimeProp(endProp)
		if err != nil {
			return nil, err
		}
		end = &split
	}
	return deriveDuration(*start, end, rawValue(comp.Props, goical.PropDuration))
}

// recurrenceProps reads a component's RRULE and EXRULE into JSCalendar
// recurrence and excluded-recurrence rules. EXRULE is the legacy RFC 2445
// exclusion rule; the calext mapping carries it across as an
// excludedRecurrenceRules entry. EXDATE / RDATE are out of scope (see the
// FromICal coverage note).
func recurrenceProps(comp *goical.Component) (rules, excluded []jscalendar.RecurrenceRule, err error) {
	for _, prop := range comp.Props.Values(goical.PropRecurrenceRule) {
		rule, err := recurrenceRuleFromRRule(prop.Value)
		if err != nil {
			return nil, nil, err
		}
		rules = append(rules, rule)
	}
	// EXRULE has no enum constant in go-ical (it is deprecated in RFC 5545); it
	// is read by its literal property name.
	for _, prop := range comp.Props.Values("EXRULE") {
		rule, err := recurrenceRuleFromRRule(prop.Value)
		if err != nil {
			return nil, nil, err
		}
		excluded = append(excluded, rule)
	}
	return rules, excluded, nil
}

// attachTimeZones reconciles an object's timeZone against the calendar's
// VTIMEZONE definitions and embeds the matching one into the object's timeZones
// map.
//
// A DTSTART;TZID= split records the timeZone as the bare TZID. When that TZID is
// the name of an IANA zone the calendar happens to also define a VTIMEZONE for,
// the IANA name is authoritative and no embedding is needed. But when the TZID
// names a custom (non-IANA) zone the calendar defines, the JSCalendar object
// must reference it through its "/"-prefixed custom identifier so the reference
// resolves in timeZones (RFC 8984, Section 4.7.2): this rewrites the object's
// timeZone to the "/"-prefixed id and embeds the definition.
//
// A custom zone the object already references ("/"-prefixed) is embedded
// directly. A reference with no matching VTIMEZONE leaves the map empty; the
// validation phase reports the unresolved reference.
func attachTimeZones(zone *jscalendar.TimeZoneId, available map[jscalendar.TimeZoneId]jscalendar.TimeZone, dst *map[jscalendar.TimeZoneId]jscalendar.TimeZone) {
	if *zone == "" {
		return
	}

	key := *zone
	if !zone.IsCustom() {
		// Prefer the IANA name: only rewrite to the custom form when the calendar
		// defines a VTIMEZONE for this TZID and the name is not a usable IANA zone.
		custom := customTimeZoneID(string(*zone))
		if _, ok := available[custom]; !ok {
			return
		}
		if _, err := time.LoadLocation(string(*zone)); err == nil {
			return
		}
		*zone = custom
		key = custom
	}

	tz, ok := available[key]
	if !ok {
		return
	}
	if *dst == nil {
		*dst = map[jscalendar.TimeZoneId]jscalendar.TimeZone{}
	}
	(*dst)[key] = tz
}
