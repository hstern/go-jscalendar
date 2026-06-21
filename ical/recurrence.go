// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hstern/go-jscalendar"
)

// This file converts an iCalendar RRULE string (RFC 5545, Section 3.3.10) into
// the structured JSCalendar [jscalendar.RecurrenceRule] (RFC 8984, Section
// 4.3.1) — the centerpiece of the FromICal conversion. RRULE is an opaque
// semicolon-separated string of "KEY=value" parts; JSCalendar replaces it with
// a typed object whose members mirror the RRULE parts one for one, with a few
// shape changes the mapping pins down:
//
//   - FREQ is lower-cased ("WEEKLY" → "weekly").
//   - BYDAY entries become [jscalendar.NDay] values, splitting the optional
//     leading ordinal ("-1SU" → {Day:"su", NthOfPeriod:-1}).
//   - BYMONTH values stay STRINGS on the JSCalendar side (a leap-month "5L" can
//     only be expressed as a string), so the RRULE integers are carried across
//     verbatim as text.
//   - WKST becomes firstDayOfWeek, lower-cased; BYSETPOS becomes bySetPosition.
//   - UNTIL becomes a LocalDateTime in the enclosing object's zone: its zone
//     designator (a trailing "Z" or the object's TZID) is dropped, since
//     JSCalendar interprets "until" in the object's own timeZone, not UTC.
//
// An unrecognized RRULE part is skipped rather than rejected, matching the
// library's lenient posture: a recurrence that uses a part this mapping does
// not model still produces a usable rule for the parts it does.

// frequencyFromRRule maps the upper-case RFC 5545 FREQ token to the lower-case
// JSCalendar [jscalendar.Frequency]. An unrecognized token is carried across
// verbatim (lower-cased) rather than rejected, since Frequency is an open
// string on the JSCalendar side.
func frequencyFromRRule(freq string) jscalendar.Frequency {
	return jscalendar.Frequency(strings.ToLower(freq))
}

// recurrenceRuleFromRRule parses one iCalendar RRULE string into a JSCalendar
// [jscalendar.RecurrenceRule]. The RRULE grammar is a ";"-separated list of
// "KEY=value" parts (RFC 5545, Section 3.3.10); each part maps to one field of
// the rule. A part with no recognized key is skipped. FREQ is the only required
// part; its absence leaves Frequency empty, which the JSCalendar marshaler
// rejects — the same strictness boundary the rest of the library applies.
func recurrenceRuleFromRRule(value string) (jscalendar.RecurrenceRule, error) {
	var rule jscalendar.RecurrenceRule
	for _, part := range strings.Split(value, ";") {
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			return jscalendar.RecurrenceRule{}, fmt.Errorf("ical: malformed RRULE part %q", part)
		}
		if err := applyRRulePart(&rule, strings.ToUpper(key), val); err != nil {
			return jscalendar.RecurrenceRule{}, err
		}
	}
	return rule, nil
}

// applyRRulePart sets the one field of rule that the given RRULE key addresses.
// Unrecognized keys are ignored.
func applyRRulePart(rule *jscalendar.RecurrenceRule, key, val string) error {
	switch key {
	case "FREQ":
		rule.Frequency = frequencyFromRRule(val)
	case "INTERVAL":
		n, err := parseUintPart(key, val)
		if err != nil {
			return err
		}
		rule.Interval = uint(n)
	case "RSCALE":
		rule.RScale = strings.ToLower(val)
	case "SKIP":
		rule.Skip = strings.ToLower(val)
	case "WKST":
		rule.FirstDayOfWeek = strings.ToLower(val)
	case "BYDAY":
		days, err := parseByDay(val)
		if err != nil {
			return err
		}
		rule.ByDay = days
	case "BYMONTHDAY":
		ints, err := parseIntList(key, val)
		if err != nil {
			return err
		}
		rule.ByMonthDay = ints
	case "BYMONTH":
		// JSCalendar byMonth values are strings (a leap month is "NL"), so the
		// RRULE integers are carried across verbatim as text.
		rule.ByMonth = splitNonEmpty(val)
	case "BYYEARDAY":
		ints, err := parseIntList(key, val)
		if err != nil {
			return err
		}
		rule.ByYearDay = ints
	case "BYWEEKNO":
		ints, err := parseIntList(key, val)
		if err != nil {
			return err
		}
		rule.ByWeekNo = ints
	case "BYHOUR":
		uints, err := parseUintList(key, val)
		if err != nil {
			return err
		}
		rule.ByHour = uints
	case "BYMINUTE":
		uints, err := parseUintList(key, val)
		if err != nil {
			return err
		}
		rule.ByMinute = uints
	case "BYSECOND":
		uints, err := parseUintList(key, val)
		if err != nil {
			return err
		}
		rule.BySecond = uints
	case "BYSETPOS":
		ints, err := parseIntList(key, val)
		if err != nil {
			return err
		}
		rule.BySetPosition = ints
	case "COUNT":
		n, err := parseUintPart(key, val)
		if err != nil {
			return err
		}
		c := uint(n)
		rule.Count = &c
	case "UNTIL":
		// UNTIL is interpreted in the enclosing object's zone on the JSCalendar
		// side, so its zone designator (a trailing "Z", or the object's TZID) is
		// dropped and the wall clock kept as a LocalDateTime. A DATE-form UNTIL
		// ("YYYYMMDD") becomes midnight that day.
		until, err := parseUntil(val)
		if err != nil {
			return err
		}
		rule.Until = &until
	default:
		// Unrecognized part: skip it, keeping the rest of the rule usable.
	}
	return nil
}

// parseByDay parses a comma-separated RRULE BYDAY list into [jscalendar.NDay]
// values. Each entry is an optional signed ordinal followed by a two-letter
// weekday code ("MO", "-1SU", "2TH"); the ordinal becomes NthOfPeriod and the
// code is lower-cased into Day.
func parseByDay(val string) ([]jscalendar.NDay, error) {
	parts := splitNonEmpty(val)
	if len(parts) == 0 {
		return nil, nil
	}
	days := make([]jscalendar.NDay, 0, len(parts))
	for _, p := range parts {
		nday, err := parseNDay(p)
		if err != nil {
			return nil, err
		}
		days = append(days, nday)
	}
	return days, nil
}

// parseNDay parses one RRULE weekday entry ("MO", "-1SU", "2TH") into an
// [jscalendar.NDay]. The trailing two letters are the weekday code; any leading
// signed digits are the ordinal occurrence within the recurrence period.
func parseNDay(p string) (jscalendar.NDay, error) {
	if len(p) < 2 {
		return jscalendar.NDay{}, fmt.Errorf("ical: malformed BYDAY entry %q", p)
	}
	code := p[len(p)-2:]
	ordinalStr := p[:len(p)-2]
	nday := jscalendar.NDay{Day: strings.ToLower(code)}
	if ordinalStr != "" {
		n, err := strconv.Atoi(ordinalStr)
		if err != nil {
			return jscalendar.NDay{}, fmt.Errorf("ical: malformed BYDAY ordinal in %q: %w", p, err)
		}
		nday.NthOfPeriod = &n
	}
	return nday, nil
}

// parseUntil parses an RRULE UNTIL value, which is a DATE-TIME (optionally
// UTC-suffixed) or a DATE, into a zone-free [jscalendar.LocalDateTime]. The zone
// designator is dropped: JSCalendar interprets "until" in the object's own
// timeZone, so folding a UTC instant in here would silently shift the boundary.
func parseUntil(val string) (jscalendar.LocalDateTime, error) {
	if len(strings.TrimSuffix(val, "Z")) == len(icalDateLayout) {
		return parseICalDate(val)
	}
	return parseICalDateTime(val)
}

// parseUintPart parses a single non-negative integer RRULE value (INTERVAL,
// COUNT).
func parseUintPart(key, val string) (uint64, error) {
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ical: malformed RRULE %s=%q: %w", key, val, err)
	}
	return n, nil
}

// parseIntList parses a comma-separated list of signed integers (BYMONTHDAY,
// BYYEARDAY, BYWEEKNO, BYSETPOS).
func parseIntList(key, val string) ([]int, error) {
	parts := splitNonEmpty(val)
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("ical: malformed RRULE %s entry %q: %w", key, p, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// parseUintList parses a comma-separated list of non-negative integers (BYHOUR,
// BYMINUTE, BYSECOND).
func parseUintList(key, val string) ([]uint, error) {
	parts := splitNonEmpty(val)
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]uint, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("ical: malformed RRULE %s entry %q: %w", key, p, err)
		}
		out = append(out, uint(n))
	}
	return out, nil
}

// splitNonEmpty splits a comma-separated list, dropping empty fields so a
// trailing comma or a doubled separator does not produce a zero-length entry.
func splitNonEmpty(val string) []string {
	raw := strings.Split(val, ",")
	out := raw[:0]
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
