// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"time"

	"github.com/hstern/go-jscalendar"
)

// This file derives a JSCalendar [jscalendar.Duration] from an iCalendar
// VEVENT/VTODO time span. The span is given either explicitly by a DURATION
// property or implicitly by a DTEND/DUE relative to DTSTART; both reduce to a
// non-negative duration on the JSCalendar side.

// utcTimeZone is the JSCalendar TimeZoneId for UTC. RFC 8984 uses the IANA
// "Etc/UTC" name, and the calext mapping carries a DATE-TIME with a trailing
// "Z" across as a wall clock plus this zone, preserving the absolute instant
// without folding the zone into the LocalDateTime.
const utcTimeZone jscalendar.TimeZoneId = "Etc/UTC"

// unixUTC returns the wall clock interpreted as a UTC instant, in seconds since
// the Unix epoch.
func unixUTC(lt jscalendar.LocalDateTime) int64 {
	return time.Date(lt.Year, time.Month(lt.Month), lt.Day, lt.Hour, lt.Minute, lt.Second, 0, time.UTC).Unix()
}

// unixInZone returns the wall clock interpreted in the named IANA zone, in
// seconds since the Unix epoch, and whether the zone could be loaded. A zone
// that is not in the host's tzdata (or a custom, non-IANA name) returns ok
// false; the caller then falls back to a wall-clock difference for any duration
// it needs, since the absolute instant is not computable here.
func unixInZone(lt jscalendar.LocalDateTime, tzid string) (int64, bool) {
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		return 0, false
	}
	return time.Date(lt.Year, time.Month(lt.Month), lt.Day, lt.Hour, lt.Minute, lt.Second, 0, loc).Unix(), true
}

// durationFromSeconds builds a non-negative JSCalendar [jscalendar.Duration]
// from a count of whole seconds, normalized into the canonical H/M/S grouping
// by the Duration formatter. A negative count is clamped to zero: a JSCalendar
// Duration is non-negative by definition (RFC 8984, Section 1.4.6), and an end
// before its start is degenerate iCalendar that maps to the zero span.
func durationFromSeconds(sec int64) jscalendar.Duration {
	if sec < 0 {
		sec = 0
	}
	return jscalendar.Duration{Seconds: uint64(sec)}
}

// durationFromDays builds a non-negative whole-day JSCalendar
// [jscalendar.Duration]. It is used for all-day (DATE-valued) spans, where the
// inclusive iCalendar DTEND convention and the absence of a time component make
// a day count the faithful unit (RFC 8984, Section 1.4.6 grammar's "D").
func durationFromDays(days int) jscalendar.Duration {
	if days < 0 {
		days = 0
	}
	return jscalendar.Duration{Days: uint64(days)}
}

// deriveDuration computes the JSCalendar duration for a component from its
// DTSTART split and an optional DTEND/DUE split, following the calext mapping:
//
//   - An explicit DURATION property (durProp non-empty) is parsed directly with
//     the JSCalendar grammar, which is a superset of the iCalendar one for the
//     forms VEVENT/VTODO use.
//   - Otherwise, when an end split is given, the duration is end − start. For an
//     all-day (DATE) span it is the whole-day difference; for a timed span it is
//     the second difference between the two absolute instants when both resolve,
//     falling back to a naive wall-clock difference when a zone did not load.
//   - With neither a DURATION nor an end, the duration is unset (nil), letting
//     the JSCalendar default of "P0D" apply.
//
// The returned pointer is nil when no duration is expressed.
func deriveDuration(start splitDateTime, end *splitDateTime, durProp string) (*jscalendar.Duration, error) {
	if durProp != "" {
		d, err := jscalendar.ParseDuration(durProp)
		if err != nil {
			return nil, err
		}
		return &d, nil
	}
	if end == nil {
		return nil, nil
	}

	if start.kind == kindDate || end.kind == kindDate {
		days := daysBetween(start.local, end.local)
		d := durationFromDays(days)
		return &d, nil
	}

	var sec int64
	if start.utcOK && end.utcOK {
		sec = end.utc - start.utc
	} else {
		// Neither instant resolved (a custom or unknown zone): fall back to the
		// wall-clock difference, which is exact when start and end share a zone —
		// the only shape RFC 5545 permits for a DTSTART/DTEND pair.
		sec = unixUTC(end.local) - unixUTC(start.local)
	}
	d := durationFromSeconds(sec)
	return &d, nil
}

// daysBetween returns the number of whole days from start to end, computed over
// the UTC interpretation of the two midnights so calendar arithmetic (month
// lengths, leap years) is handled by the standard library.
func daysBetween(start, end jscalendar.LocalDateTime) int {
	const secondsPerDay = 24 * 60 * 60
	return int((unixUTC(end) - unixUTC(start)) / secondsPerDay)
}
