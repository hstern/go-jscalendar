// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file converts an iCalendar VTIMEZONE component (RFC 5545, Section 3.6.5)
// into a JSCalendar [jscalendar.TimeZone] (RFC 8984, Section 4.7.2). The
// conversion is best-effort over the common shape: the TZID, and the STANDARD /
// DAYLIGHT sub-components as [jscalendar.TimeZoneRule] entries carrying their
// DTSTART (as the rule's start), TZOFFSETFROM / TZOFFSETTO, TZNAME, and any
// RRULE that repeats the transition.
//
// Coverage boundary (documented in the FromICal godoc): RDATE-based transitions
// and the long tail of VTIMEZONE metadata beyond TZID are not modeled. The
// rules that are carried are sufficient to round-trip the standard biannual
// DST pattern that calendar data overwhelmingly uses.

// timeZoneFromVTimezone converts one VTIMEZONE component into a JSCalendar
// [jscalendar.TimeZone] and the custom, "/"-prefixed [jscalendar.TimeZoneId]
// that keys it in an object's timeZones map. The map key is the TZID prefixed
// with "/" — JSCalendar's marker for a per-object, non-IANA zone definition;
// the TimeZone's own tzId carries the TZID verbatim.
func timeZoneFromVTimezone(comp *goical.Component) (jscalendar.TimeZoneId, jscalendar.TimeZone, error) {
	tzid := rawValue(comp.Props, goical.PropTimezoneID)
	tz := jscalendar.TimeZone{TzID: tzid}

	for _, child := range comp.Children {
		rule, err := timeZoneRuleFromComponent(child)
		if err != nil {
			return "", jscalendar.TimeZone{}, err
		}
		switch child.Name {
		case goical.CompTimezoneStandard:
			tz.Standard = append(tz.Standard, rule)
		case goical.CompTimezoneDaylight:
			tz.Daylight = append(tz.Daylight, rule)
		default:
			// Unrecognized VTIMEZONE sub-component: skip it.
		}
	}

	return customTimeZoneID(tzid), tz, nil
}

// timeZoneRuleFromComponent converts a STANDARD or DAYLIGHT sub-component into a
// [jscalendar.TimeZoneRule]: its DTSTART becomes the rule's start, its
// TZOFFSETFROM / TZOFFSETTO become the offsets, each TZNAME becomes a name, and
// an RRULE becomes a recurrence rule that repeats the transition.
func timeZoneRuleFromComponent(comp *goical.Component) (jscalendar.TimeZoneRule, error) {
	rule := jscalendar.TimeZoneRule{
		OffsetFrom: offsetValue(rawValue(comp.Props, goical.PropTimezoneOffsetFrom)),
		OffsetTo:   offsetValue(rawValue(comp.Props, goical.PropTimezoneOffsetTo)),
	}

	if startProp := comp.Props.Get(goical.PropDateTimeStart); startProp != nil {
		start, err := parseICalDateTime(startProp.Value)
		if err != nil {
			return jscalendar.TimeZoneRule{}, err
		}
		rule.Start = &start
	}

	for _, nameProp := range comp.Props.Values(goical.PropTimezoneName) {
		if rule.Names == nil {
			rule.Names = map[string]bool{}
		}
		rule.Names[nameProp.Value] = true
	}

	if rruleProp := comp.Props.Get(goical.PropRecurrenceRule); rruleProp != nil {
		rec, err := recurrenceRuleFromRRule(rruleProp.Value)
		if err != nil {
			return jscalendar.TimeZoneRule{}, err
		}
		rule.RecurrenceRules = []jscalendar.RecurrenceRule{rec}
	}

	return rule, nil
}

// customTimeZoneID returns the JSCalendar custom-zone identifier for an
// iCalendar TZID: the TZID with a leading "/" so it resolves against the
// object's embedded timeZones map rather than the IANA database (RFC 8984,
// Section 4.7.2). A TZID that already begins with "/" is returned unchanged.
func customTimeZoneID(tzid string) jscalendar.TimeZoneId {
	if len(tzid) > 0 && tzid[0] == '/' {
		return jscalendar.TimeZoneId(tzid)
	}
	return jscalendar.TimeZoneId("/" + tzid)
}

// offsetValue converts an iCalendar UTC-OFFSET ("+HHMM", "-HHMMSS") into the
// JSCalendar signed "+HH:MM" / "-HH:MM" UTCOffset shape by inserting the
// colon. A value that is not in the expected fixed-width form is returned
// unchanged, leaving any normalization to the validation boundary.
func offsetValue(s string) string {
	if len(s) != 5 && len(s) != 7 {
		return s
	}
	sign := s[0]
	if sign != '+' && sign != '-' {
		return s
	}
	out := string(sign) + s[1:3] + ":" + s[3:5]
	if len(s) == 7 {
		out += ":" + s[5:7]
	}
	return out
}
