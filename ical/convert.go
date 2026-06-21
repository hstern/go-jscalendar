// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"strconv"
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file holds the low-level value conversions shared by the VEVENT and
// VTODO mappings in from_ical.go: the date-time split that is the centerpiece
// of the iCalendar-to-JSCalendar correspondence, the duration derivations, and
// the small helpers for reading single-valued properties.
//
// The conversions work directly on the go-ical [goical.Prop] / [goical.Params]
// surface rather than the higher-level goical.Prop.DateTime helper. That helper
// resolves a TZID into a Go [time.Location] via time.LoadLocation, which both
// imposes a tzdata dependency at runtime and cannot represent the iCalendar
// TZID verbatim — exactly the information a JSCalendar object must preserve as
// its timeZone. Reading the raw value and the TZID parameter keeps the wall
// clock and the zone name independent, which is what the LocalDateTime +
// timeZone split requires.

// dateTimeKind classifies the four forms an iCalendar DATE-TIME or DATE
// property value can take, which drive how it maps onto a JSCalendar object.
//
// The zero value is kindDate rather than an "unknown" sentinel: every
// [splitDateTime] is produced by [splitDateTimeProp], which always sets kind
// explicitly, so a zero-valued kind never escapes as a meaningful default.
type dateTimeKind int

const (
	// kindDate is an RFC 5545 DATE value ("YYYYMMDD"): an all-day, time-of-day-
	// insignificant value. It maps to a midnight LocalDateTime with the object's
	// showWithoutTime set.
	kindDate dateTimeKind = iota
	// kindUTC is a DATE-TIME value with a trailing "Z": an absolute instant in
	// UTC. JSCalendar carries the wall clock as a LocalDateTime and the zone
	// separately, so this maps to the wall clock plus the "Etc/UTC" timeZone.
	kindUTC
	// kindLocal is a DATE-TIME value with neither a "Z" nor a TZID: a floating
	// local time. It maps to a LocalDateTime with no timeZone.
	kindLocal
	// kindZoned is a DATE-TIME value carrying a ";TZID=" parameter: a wall clock
	// in a named zone. It maps to a LocalDateTime plus that TZID as the timeZone.
	kindZoned
)

// splitDateTime is the result of decomposing an iCalendar DATE / DATE-TIME
// property into its JSCalendar parts: the zone-free wall clock and, when the
// source named a zone, the [jscalendar.TimeZoneId] that governs it.
type splitDateTime struct {
	// local is the zone-free wall-clock value. For a DATE source it is midnight
	// on that day.
	local jscalendar.LocalDateTime
	// timeZone is the zone the wall clock is interpreted in, or empty for a
	// floating (kindLocal) or all-day (kindDate) value.
	timeZone jscalendar.TimeZoneId
	// kind records which of the four source forms produced this split, so the
	// caller can set showWithoutTime for an all-day value.
	kind dateTimeKind
	// utc is the absolute instant, populated only for kindUTC and kindZoned with
	// a loadable zone; it is used to derive a DTEND-based duration. It is the
	// wall clock interpreted in timeZone (or UTC), in seconds since the Unix
	// epoch. ok reports whether utc could be computed.
	utc   int64
	utcOK bool
}

// splitDateTimeProp decomposes an iCalendar DATE / DATE-TIME property into its
// JSCalendar LocalDateTime and (optional) TimeZoneId per the calext
// jscalendar-icalendar mapping:
//
//   - A DATE value ("YYYYMMDD") yields a midnight LocalDateTime and no zone;
//     the caller sets showWithoutTime.
//   - A DATE-TIME ending in "Z" yields the wall clock plus the "Etc/UTC" zone
//     (the spec's canonical name for UTC), so the absolute instant is preserved
//     without folding the zone into the wall clock.
//   - A DATE-TIME with a ";TZID=" parameter yields the wall clock plus that
//     TZID as the zone — the DTSTART;TZID= split that is the load-bearing case.
//   - A bare DATE-TIME (no "Z", no TZID) yields a floating LocalDateTime with
//     no zone.
func splitDateTimeProp(prop *goical.Prop) (splitDateTime, error) {
	value := prop.Value
	tzid := prop.Params.Get(goical.ParamTimezoneID)
	isDate := prop.ValueType() == goical.ValueDate || (prop.Params.Get(goical.ParamValue) == "" && len(value) == len(icalDateLayout))

	if isDate {
		local, err := parseICalDate(value)
		if err != nil {
			return splitDateTime{}, err
		}
		return splitDateTime{local: local, kind: kindDate}, nil
	}

	utc := strings.HasSuffix(value, "Z")
	local, err := parseICalDateTime(value)
	if err != nil {
		return splitDateTime{}, err
	}

	switch {
	case utc:
		s := splitDateTime{local: local, timeZone: utcTimeZone, kind: kindUTC}
		s.utc, s.utcOK = unixUTC(local), true
		return s, nil
	case tzid != "":
		s := splitDateTime{local: local, timeZone: jscalendar.TimeZoneId(tzid), kind: kindZoned}
		// Best-effort absolute instant for duration math: only available when
		// the zone loads from the host's tzdata. When it does not, a DTEND-based
		// duration falls back to a wall-clock difference (see deriveDuration).
		s.utc, s.utcOK = unixInZone(local, tzid)
		return s, nil
	default:
		return splitDateTime{local: local, kind: kindLocal}, nil
	}
}

// icalDateLayout and icalDateTimeLayout are the fixed RFC 5545 forms, with no
// separators, that an iCalendar DATE and DATE-TIME value carry on the wire.
const (
	icalDateLayout     = "20060102"
	icalDateTimeLayout = "20060102T150405"
)

// parseICalDate parses an RFC 5545 DATE value ("YYYYMMDD") into a midnight
// [jscalendar.LocalDateTime].
func parseICalDate(s string) (jscalendar.LocalDateTime, error) {
	if len(s) != len(icalDateLayout) {
		return jscalendar.LocalDateTime{}, fmt.Errorf("ical: malformed DATE %q", s)
	}
	year, err1 := atoi(s[0:4])
	month, err2 := atoi(s[4:6])
	day, err3 := atoi(s[6:8])
	if err := firstErr(err1, err2, err3); err != nil {
		return jscalendar.LocalDateTime{}, fmt.Errorf("ical: malformed DATE %q: %w", s, err)
	}
	return jscalendar.LocalDateTime{Year: year, Month: month, Day: day}, nil
}

// parseICalDateTime parses an RFC 5545 DATE-TIME value ("YYYYMMDDTHHMMSS",
// optionally with a trailing "Z") into a zone-free [jscalendar.LocalDateTime].
// The trailing "Z", when present, is consumed: the zone it denotes is recorded
// by the caller as a separate timeZone, never folded into the wall clock.
func parseICalDateTime(s string) (jscalendar.LocalDateTime, error) {
	s = strings.TrimSuffix(s, "Z")
	if len(s) != len(icalDateTimeLayout) {
		return jscalendar.LocalDateTime{}, fmt.Errorf("ical: malformed DATE-TIME %q", s)
	}
	year, err1 := atoi(s[0:4])
	month, err2 := atoi(s[4:6])
	day, err3 := atoi(s[6:8])
	hour, err4 := atoi(s[9:11])
	minute, err5 := atoi(s[11:13])
	second, err6 := atoi(s[13:15])
	if err := firstErr(err1, err2, err3, err4, err5, err6); err != nil {
		return jscalendar.LocalDateTime{}, fmt.Errorf("ical: malformed DATE-TIME %q: %w", s, err)
	}
	if s[8] != 'T' {
		return jscalendar.LocalDateTime{}, fmt.Errorf("ical: malformed DATE-TIME %q: missing T separator", s)
	}
	return jscalendar.LocalDateTime{
		Year: year, Month: month, Day: day,
		Hour: hour, Minute: minute, Second: second,
	}, nil
}

// atoi parses an all-digit field, rejecting signs and any non-digit byte that
// strconv.Atoi would tolerate.
func atoi(s string) (int, error) {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("non-digit in %q", s)
		}
	}
	return strconv.Atoi(s)
}

// firstErr returns the first non-nil error among its arguments, or nil.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// textValue returns the unescaped text value of the named property, or "" when
// the property is absent. iCalendar TEXT values escape commas, semicolons, and
// newlines; goical.Prop.Text reverses that escaping.
func textValue(props goical.Props, name string) string {
	prop := props.Get(name)
	if prop == nil {
		return ""
	}
	// Text() reverses RFC 5545 TEXT escaping. A malformed escape yields an
	// error; fall back to the raw value rather than dropping the property.
	if v, err := prop.Text(); err == nil {
		return v
	}
	return prop.Value
}

// rawValue returns the raw (un-decoded) value of the named property, or "" when
// the property is absent. It is used for values that are not RFC 5545 TEXT —
// URIs, integers — where the escaping reversal of textValue would be wrong.
func rawValue(props goical.Props, name string) string {
	prop := props.Get(name)
	if prop == nil {
		return ""
	}
	return prop.Value
}
