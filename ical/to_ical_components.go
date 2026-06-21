// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"errors"
	"fmt"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file builds the nested iCalendar components ToICal emits — the VALARM
// for each Alert and the VTIMEZONE for each embedded TimeZone — the inverses of
// alarm.go and timezone.go respectively.

// errUnsupportedType is returned by ToICal when an argument is not one of the
// concrete JSCalendar object pointer types the conversion handles (a
// *jscalendar.Event, *jscalendar.Task, or *jscalendar.Group). It is a sentinel:
// callers may test for it with [errors.Is].
var errUnsupportedType = errors.New("jscalendar/ical: unsupported object type")

// errControlChar is returned by ToICal when a JSCalendar string value carries a
// control character (a CR, LF, or other C0 byte) that would forge an iCalendar
// content line if emitted into a property or parameter value. It is a sentinel:
// callers may test for it with [errors.Is].
var errControlChar = errors.New("jscalendar/ical: value contains a control character")

// checkNoControlChars walks a component tree and fails on the first property or
// parameter value containing a control character. iCalendar content lines are
// CRLF-delimited, so an embedded CR or LF in a value would inject a forged line;
// other C0 control bytes are likewise rejected as malformed. The walk is the
// conversion's last step, so it covers every property and parameter ToICal sets
// regardless of which helper produced it.
func checkNoControlChars(comp *goical.Component) error {
	for name, props := range comp.Props {
		for i := range props {
			prop := &props[i]
			if hasControlChar(prop.Value) {
				return fmt.Errorf("%w: property %s", errControlChar, name)
			}
			for paramName, values := range prop.Params {
				for _, v := range values {
					if hasControlChar(v) {
						return fmt.Errorf("%w: parameter %s of %s", errControlChar, paramName, name)
					}
				}
			}
		}
	}
	for _, child := range comp.Children {
		if err := checkNoControlChars(child); err != nil {
			return err
		}
	}
	return nil
}

// hasControlChar reports whether s contains an ASCII control character (a C0
// byte below 0x20, or DEL 0x7F), excluding the horizontal tab, which RFC 5545
// permits in a TEXT value. CR and LF are the injection-relevant cases; the rest
// are rejected as malformed rather than silently emitted.
func hasControlChar(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c == '\t' {
			continue
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// alertsToVAlarms appends one VALARM child to comp for each alert, in alert-id
// order so the output is deterministic — the inverse of alertsFromVAlarms.
func alertsToVAlarms(comp *goical.Component, alerts map[jscalendar.Id]jscalendar.Alert) error {
	for _, id := range sortedIDs(alerts) {
		alarm, err := valarmFromAlert(alerts[id])
		if err != nil {
			return err
		}
		comp.Children = append(comp.Children, alarm)
	}
	return nil
}

// valarmFromAlert builds a VALARM component from a JSCalendar [jscalendar.Alert]
// — the inverse of alertFromVAlarm. The action is upper-cased back to the
// iCalendar form; an alert with no action defaults to DISPLAY, the only action
// RFC 5545 lets a VALARM omit nothing else for. The trigger is written from the
// alert's offset or absolute kind.
func valarmFromAlert(alert jscalendar.Alert) (*goical.Component, error) {
	comp := goical.NewComponent(goical.CompAlarm)

	action := "DISPLAY"
	if alert.Action != "" {
		action = upperASCII(alert.Action)
	}
	comp.Props.SetText(goical.PropAction, action)

	prop, err := triggerProp(alert.Trigger)
	if err != nil {
		return nil, err
	}
	if prop != nil {
		comp.Props.Set(prop)
	}

	// RFC 5545 requires a DISPLAY VALARM to carry a DESCRIPTION; supply a minimal
	// one so the emitted alarm is structurally valid. JSCalendar alerts have no
	// per-alert description field, so this text is synthesized, not round-tripped.
	if action == "DISPLAY" {
		comp.Props.SetText(goical.PropDescription, "Reminder")
	}

	return comp, nil
}

// triggerProp builds a VALARM TRIGGER property from a JSCalendar trigger — the
// inverse of triggerFromProp. An [jscalendar.OffsetTrigger] becomes a DURATION
// trigger, with relativeTo "end" carried back to the RELATED=END parameter; an
// [jscalendar.AbsoluteTrigger] becomes a DATE-TIME (UTC) trigger. A zero trigger
// yields no property (nil), so a triggerless alert maps to a triggerless alarm.
func triggerProp(trigger jscalendar.Trigger) (*goical.Prop, error) {
	switch v := trigger.Value().(type) {
	case jscalendar.OffsetTrigger:
		prop := goical.NewProp(goical.PropTrigger)
		prop.Value = v.Offset.String()
		if v.RelativeTo == "end" {
			prop.Params.Set(goical.ParamRelated, "END")
		}
		return prop, nil
	case jscalendar.AbsoluteTrigger:
		prop := goical.NewProp(goical.PropTrigger)
		prop.SetValueType(goical.ValueDateTime)
		prop.Value = formatICalUTC(v.When)
		return prop, nil
	case nil:
		// A zero trigger (no recognized kind, no preserved bytes) has nothing to
		// emit; an unrecognized-kind trigger preserved as raw JSON likewise has no
		// iCalendar TRIGGER form and is dropped (documented on ToICal).
		return nil, nil
	default:
		return nil, fmt.Errorf("ical: unexpected trigger kind %T", v)
	}
}

// vtimezoneFromTimeZone builds a VTIMEZONE component from a JSCalendar
// [jscalendar.TimeZone] — the inverse of timeZoneFromVTimezone. The TZID is the
// zone's own tzId; each standard and daylight rule becomes a STANDARD or
// DAYLIGHT sub-component.
func vtimezoneFromTimeZone(tz jscalendar.TimeZone) (*goical.Component, error) {
	comp := goical.NewComponent(goical.CompTimezone)
	comp.Props.SetText(goical.PropTimezoneID, tz.TzID)

	for _, rule := range tz.Standard {
		sub, err := timezoneRuleToComponent(goical.CompTimezoneStandard, rule)
		if err != nil {
			return nil, err
		}
		comp.Children = append(comp.Children, sub)
	}
	for _, rule := range tz.Daylight {
		sub, err := timezoneRuleToComponent(goical.CompTimezoneDaylight, rule)
		if err != nil {
			return nil, err
		}
		comp.Children = append(comp.Children, sub)
	}

	return comp, nil
}

// timezoneRuleToComponent builds a STANDARD or DAYLIGHT sub-component from a
// JSCalendar [jscalendar.TimeZoneRule] — the inverse of
// timeZoneRuleFromComponent. The rule's start becomes DTSTART, its offsets
// become TZOFFSETFROM / TZOFFSETTO (the colon re-stripped from the JSCalendar
// "+HH:MM" shape), each name becomes a TZNAME, and the first recurrence rule
// becomes an RRULE.
func timezoneRuleToComponent(name string, rule jscalendar.TimeZoneRule) (*goical.Component, error) {
	comp := goical.NewComponent(name)

	if rule.Start != nil {
		prop := goical.NewProp(goical.PropDateTimeStart)
		prop.Value = formatICalDateTime(*rule.Start)
		comp.Props.Set(prop)
	}
	if rule.OffsetFrom != "" {
		comp.Props.SetText(goical.PropTimezoneOffsetFrom, icalOffset(rule.OffsetFrom))
	}
	if rule.OffsetTo != "" {
		comp.Props.SetText(goical.PropTimezoneOffsetTo, icalOffset(rule.OffsetTo))
	}
	for _, n := range sortedTrueKeys(rule.Names) {
		prop := goical.NewProp(goical.PropTimezoneName)
		prop.Value = n
		comp.Props.Add(prop)
	}
	for _, rec := range rule.RecurrenceRules {
		value, err := rruleString(rec)
		if err != nil {
			return nil, err
		}
		prop := goical.NewProp(goical.PropRecurrenceRule)
		prop.Value = value
		comp.Props.Add(prop)
	}

	return comp, nil
}

// icalOffset converts a JSCalendar UTCOffset ("+HH:MM" / "-HH:MM[:SS]") back to
// the iCalendar UTC-OFFSET form ("+HHMM" / "-HHMMSS") by removing the colons —
// the inverse of offsetValue. A value not in the expected shape is returned
// unchanged, leaving any oddity to the encoder.
func icalOffset(offset string) string {
	out := make([]byte, 0, len(offset))
	for i := range len(offset) {
		if offset[i] != ':' {
			out = append(out, offset[i])
		}
	}
	return string(out)
}

// upperASCII upper-cases an ASCII string. Action values are ASCII tokens, so a
// byte-wise fold avoids the Unicode special-casing of strings.ToUpper.
func upperASCII(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'a' && out[i] <= 'z' {
			out[i] -= 'a' - 'A'
		}
	}
	return string(out)
}
