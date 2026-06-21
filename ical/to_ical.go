// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file implements ToICal: the JSCalendar-to-iCalendar direction of the
// conversion, the inverse of from_ical.go. It walks each top-level JSCalendar
// object into one iCalendar component — an Event into a VEVENT, a Task into a
// VTODO — and emits the embedded timeZones of any referenced custom zone as
// VTIMEZONE siblings. A Group expands its entries into their own components.
//
// The mapping is the reverse of the IETF calext working group's "JSCalendar:
// Converting from and to iCalendar" property correspondence; it reuses the
// value-level helpers (date-time formatting, the offset shape) that from_ical.go
// also relies on, so the two directions stay symmetric. The doc comment on
// ToICal in ical.go records the lossy edges where the round trip is not exact.

// ToICal is implemented in this file; the published doc comment lives on the
// declaration in ical.go alongside ErrNotImplemented, which ToICal no longer
// returns now that the conversion is built.
//
// Every produced VEVENT/VTODO carries the mandatory DTSTAMP and UID so the
// result encodes through goical.NewEncoder without the encoder's own
// exactly-one-of check rejecting it; the calendar carries VERSION and PRODID
// for the same reason.
func toICal(objs ...any) (*goical.Calendar, error) {
	cal := goical.NewCalendar()
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, prodID)

	// A custom ("/"-prefixed) zone is defined once per calendar even when several
	// components reference it; collect the union and emit each VTIMEZONE once,
	// ahead of the components, matching the layout FromICal consumes.
	zones := map[jscalendar.TimeZoneId]jscalendar.TimeZone{}

	for _, obj := range objs {
		if err := appendObject(cal, obj, zones); err != nil {
			return nil, err
		}
	}

	for _, id := range sortedZoneIDs(zones) {
		comp, err := vtimezoneFromTimeZone(zones[id])
		if err != nil {
			return nil, err
		}
		// VTIMEZONE precedes the components that reference it.
		cal.Children = append([]*goical.Component{comp}, cal.Children...)
	}

	// Reject any control character that survived into a property or parameter
	// value before returning. This is defense in depth: go-ical's encoder rejects
	// a CR/LF in a property VALUE, but it does NOT reject one in a PARAMETER value
	// (a CN or TZID), where an embedded CRLF would forge a content line in the
	// consumer's view — an iCalendar injection. Failing the conversion here keeps
	// ToICal's output safe regardless of the encoder's parameter handling.
	if err := checkNoControlChars(cal.Component); err != nil {
		return nil, err
	}

	return cal, nil
}

// prodID is the PRODID stamped on calendars ToICal produces. It names the
// library, not any consuming product, so the output is attributable without
// disclosing the caller.
const prodID = "-//go-jscalendar//EN"

// appendObject converts one top-level JSCalendar object into the component(s)
// it maps to and appends them to the calendar, accumulating any custom zones it
// references into zones for later VTIMEZONE emission. A Group recurses into its
// entries.
func appendObject(cal *goical.Calendar, obj any, zones map[jscalendar.TimeZoneId]jscalendar.TimeZone) error {
	switch v := obj.(type) {
	case *jscalendar.Event:
		if v == nil {
			return fmt.Errorf("%w: nil *jscalendar.Event", errUnsupportedType)
		}
		comp, err := veventFromEvent(v)
		if err != nil {
			return err
		}
		collectObjectZones(v.TimeZones, zones)
		cal.Children = append(cal.Children, comp)
	case *jscalendar.Task:
		if v == nil {
			return fmt.Errorf("%w: nil *jscalendar.Task", errUnsupportedType)
		}
		comp, err := vtodoFromTask(v)
		if err != nil {
			return err
		}
		collectObjectZones(v.TimeZones, zones)
		cal.Children = append(cal.Children, comp)
	case *jscalendar.Group:
		if v == nil {
			return fmt.Errorf("%w: nil *jscalendar.Group", errUnsupportedType)
		}
		return appendGroup(cal, v, zones)
	case nil:
		return fmt.Errorf("%w: nil object", errUnsupportedType)
	default:
		return fmt.Errorf("%w: %T", errUnsupportedType, obj)
	}
	return nil
}

// appendGroup expands a Group's entries — each an Event or Task carried as raw
// JSON — into their own components. The entry bytes are decoded with the parent
// package's Parse so the same type dispatch ToICal applies at the top level
// applies to members.
func appendGroup(cal *goical.Calendar, group *jscalendar.Group, zones map[jscalendar.TimeZoneId]jscalendar.TimeZone) error {
	for i, entry := range group.Entries {
		obj, err := jscalendar.Parse(entry)
		if err != nil {
			return fmt.Errorf("ical: convert Group entry %d: %w", i, err)
		}
		if err := appendObject(cal, obj, zones); err != nil {
			return fmt.Errorf("ical: convert Group entry %d: %w", i, err)
		}
	}
	return nil
}

// collectObjectZones unions an object's embedded timeZones into the
// calendar-wide set. Keys are the custom "/"-prefixed identifiers; a later
// definition for the same id overwrites an earlier one, which is harmless since
// the same id denotes the same zone within a single conversion.
func collectObjectZones(src, dst map[jscalendar.TimeZoneId]jscalendar.TimeZone) {
	for id, tz := range src {
		dst[id] = tz
	}
}

// sortedZoneIDs returns the zone identifiers in sorted order so VTIMEZONE
// emission is deterministic regardless of Go's map iteration order.
func sortedZoneIDs(zones map[jscalendar.TimeZoneId]jscalendar.TimeZone) []jscalendar.TimeZoneId {
	ids := make([]jscalendar.TimeZoneId, 0, len(zones))
	for id := range zones {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// veventFromEvent maps a [*jscalendar.Event] onto a VEVENT component: the common
// properties, then the event-specific start/duration/status and recurrence.
func veventFromEvent(event *jscalendar.Event) (*goical.Component, error) {
	comp := goical.NewComponent(goical.CompEvent)
	if err := commonPropsToComp(comp, &commonSource{
		uid:          event.UID,
		title:        event.Title,
		description:  event.Description,
		sequence:     event.Sequence,
		created:      event.Created,
		updated:      event.Updated,
		privacy:      event.Privacy,
		keywords:     event.Keywords,
		locations:    event.Locations,
		participants: event.Participants,
		links:        event.Links,
		alerts:       event.Alerts,
	}); err != nil {
		return nil, err
	}

	if event.Start != nil {
		comp.Props.Set(dateTimeProp(goical.PropDateTimeStart, *event.Start, event.TimeZone, event.ShowWithoutTime))
	}
	if event.Duration != nil && !event.Duration.IsZero() {
		comp.Props.Set(durationProp(*event.Duration))
	}
	if event.Status != "" {
		comp.Props.SetText(goical.PropStatus, strings.ToUpper(event.Status))
	}

	if err := recurrenceToProps(comp, event.RecurrenceRules, event.ExcludedRecurrenceRules); err != nil {
		return nil, err
	}

	return comp, nil
}

// vtodoFromTask maps a [*jscalendar.Task] onto a VTODO component: the common
// properties, then the task-specific start/due/estimatedDuration/progress and
// recurrence.
func vtodoFromTask(task *jscalendar.Task) (*goical.Component, error) {
	comp := goical.NewComponent(goical.CompToDo)
	if err := commonPropsToComp(comp, &commonSource{
		uid:          task.UID,
		title:        task.Title,
		description:  task.Description,
		sequence:     task.Sequence,
		created:      task.Created,
		updated:      task.Updated,
		privacy:      task.Privacy,
		keywords:     task.Keywords,
		locations:    task.Locations,
		participants: task.Participants,
		links:        task.Links,
		alerts:       task.Alerts,
	}); err != nil {
		return nil, err
	}

	if task.Start != nil {
		comp.Props.Set(dateTimeProp(goical.PropDateTimeStart, *task.Start, task.TimeZone, task.ShowWithoutTime))
	}
	if task.Due != nil {
		comp.Props.Set(dateTimeProp(goical.PropDue, *task.Due, task.TimeZone, task.ShowWithoutTime))
	}
	if task.EstimatedDuration != nil && !task.EstimatedDuration.IsZero() {
		comp.Props.Set(durationProp(*task.EstimatedDuration))
	}
	if task.PercentComplete != nil {
		// PERCENT-COMPLETE is an INTEGER property; set the raw value so no spurious
		// VALUE=TEXT parameter is added.
		pct := goical.NewProp(goical.PropPercentComplete)
		pct.Value = strconv.FormatUint(uint64(*task.PercentComplete), 10)
		comp.Props.Set(pct)
	}
	if task.Progress != "" {
		comp.Props.SetText(goical.PropStatus, strings.ToUpper(task.Progress))
	}

	if err := recurrenceToProps(comp, task.RecurrenceRules, task.ExcludedRecurrenceRules); err != nil {
		return nil, err
	}

	return comp, nil
}

// recurrenceToProps writes a component's RRULE properties from the recurrence
// rules and its EXRULE properties from the excluded rules. The calext mapping
// carries excludedRecurrenceRules back to the (RFC 5545-deprecated, but
// FromICal-recognized) EXRULE; the symmetry keeps iCal→JSCalendar→iCal exact
// for the exclusion.
func recurrenceToProps(comp *goical.Component, rules, excluded []jscalendar.RecurrenceRule) error {
	for _, rule := range rules {
		value, err := rruleString(rule)
		if err != nil {
			return err
		}
		prop := goical.NewProp(goical.PropRecurrenceRule)
		prop.Value = value
		comp.Props.Add(prop)
	}
	for _, rule := range excluded {
		value, err := rruleString(rule)
		if err != nil {
			return err
		}
		prop := goical.NewProp(exrulePropName)
		prop.Value = value
		comp.Props.Add(prop)
	}
	return nil
}

// exrulePropName is the literal EXRULE property name. go-ical has no enum
// constant for it (RFC 5545 deprecated the property), so both directions read
// and write it by name; FromICal's recurrenceProps consumes the same literal.
const exrulePropName = "EXRULE"
