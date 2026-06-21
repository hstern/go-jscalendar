// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file converts an iCalendar VALARM component (RFC 5545, Section 3.6.6)
// into a JSCalendar [jscalendar.Alert] (RFC 8984, Section 4.5.2). The two
// load-bearing parts are the trigger and the action:
//
//   - TRIGGER carries either a DURATION (relative to the enclosing component's
//     start or, with RELATED=END, its end) or a DATE-TIME (an absolute instant).
//     The first becomes an OffsetTrigger whose offset is a SignedDuration; the
//     second becomes an AbsoluteTrigger at a UTCDateTime.
//   - ACTION ("DISPLAY", "EMAIL", "AUDIO") is lower-cased into the JSCalendar
//     action. JSCalendar registers only "display" and "email"; an unrecognized
//     action round-trips as-is, since the action member is open.

// alertFromVAlarm converts one VALARM component into a JSCalendar
// [jscalendar.Alert]. A VALARM with no TRIGGER yields an Alert with a zero
// trigger (omitted on marshal); the action defaults to the lower-cased ACTION
// when present.
func alertFromVAlarm(comp *goical.Component) (jscalendar.Alert, error) {
	alert := jscalendar.Alert{}

	if action := rawValue(comp.Props, goical.PropAction); action != "" {
		alert.Action = strings.ToLower(action)
	}

	trigger, ok, err := triggerFromProp(comp.Props.Get(goical.PropTrigger))
	if err != nil {
		return jscalendar.Alert{}, err
	}
	if ok {
		alert.Trigger = trigger
	}

	return alert, nil
}

// triggerFromProp converts a VALARM TRIGGER property into a JSCalendar
// [jscalendar.Trigger]. It returns ok false when prop is nil (no trigger to
// set). A TRIGGER whose VALUE is DATE-TIME (or whose value carries a date-time
// shape) becomes an AbsoluteTrigger; otherwise it is a DURATION and becomes an
// OffsetTrigger, with RELATED=END mapped to relativeTo "end".
func triggerFromProp(prop *goical.Prop) (jscalendar.Trigger, bool, error) {
	if prop == nil {
		return jscalendar.Trigger{}, false, nil
	}

	if prop.ValueType() == goical.ValueDateTime {
		split, err := splitDateTimeProp(prop)
		if err != nil {
			return jscalendar.Trigger{}, false, err
		}
		// An absolute trigger fires at a fixed UTC instant; RFC 5545 requires the
		// DATE-TIME form of a TRIGGER to be UTC. The wall clock is taken as the
		// UTC instant regardless of any (non-conformant) zone designator.
		// LocalDateTime and UTCDateTime share a field layout, so the wall clock
		// converts directly.
		when := jscalendar.UTCDateTime(split.local)
		return jscalendar.NewTrigger(jscalendar.AbsoluteTrigger{When: when}), true, nil
	}

	offset, err := jscalendar.ParseSignedDuration(prop.Value)
	if err != nil {
		return jscalendar.Trigger{}, false, err
	}
	relativeTo := ""
	if strings.EqualFold(prop.Params.Get(goical.ParamRelated), "END") {
		relativeTo = "end"
	}
	return jscalendar.NewTrigger(jscalendar.OffsetTrigger{Offset: offset, RelativeTo: relativeTo}), true, nil
}
