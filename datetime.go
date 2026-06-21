// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
)

// This file holds the parsing, formatting, and validation shared by the
// [LocalDateTime] (RFC 8984, Section 1.4.4) and [UTCDateTime] (Section
// 1.4.5) value types. Both grammars are the fixed-width form
// "YYYY-MM-DDTHH:MM:SS"; UTCDateTime appends a literal "Z".
//
// The parser is hand-written rather than delegated to time.Parse because
// the spec's grammar is stricter than any reference layout time.Parse
// will enforce: it forbids fractional seconds, forbids any UTC offset,
// requires exact two-digit zero padding on every component, and (for
// LocalDateTime) forbids the trailing "Z" that time.Parse with a "Z07:00"
// layout would happily consume. Enforcing those rules through time.Parse
// would mean post-hoc string inspection anyway, so a direct scan is both
// clearer and faster.

const (
	// localDateTimeLen is the length of "YYYY-MM-DDTHH:MM:SS".
	localDateTimeLen = 19
	// utcDateTimeLen is localDateTimeLen plus the trailing "Z".
	utcDateTimeLen = localDateTimeLen + 1
)

var (
	errBadLength     = errors.New("wrong length for YYYY-MM-DDTHH:MM:SS form")
	errBadSeparator  = errors.New("malformed separators (want \"-\", \"T\", \":\")")
	errNotDigit      = errors.New("non-digit where a digit is required")
	errMissingZ      = errors.New("UTCDateTime must end in \"Z\"")
	errYearRange     = errors.New("year out of range 0001..9999")
	errMonthRange    = errors.New("month out of range 1..12")
	errDayRange      = errors.New("day out of range for month")
	errHourRange     = errors.New("hour out of range 0..23")
	errMinuteRange   = errors.New("minute out of range 0..59")
	errSecondRange   = errors.New("second out of range 0..59")
	errNotJSONString = errors.New("expected a JSON string")
)

// dateTime is the six-field calendar/wall-clock value shared by
// [LocalDateTime] and [UTCDateTime]. The two public types are distinct
// Go types (so a floating local time can never be silently used where an
// absolute UTC instant is required), but they share an identical field
// layout, so the parse/format/validate plumbing operates on this one
// internal currency to avoid a six-value tuple threading through every
// helper.
type dateTime struct {
	year   int
	month  int
	day    int
	hour   int
	minute int
	second int
}

// scanDateTime parses the fixed-width date-time form. When requireZ is
// true the input must end in a literal "Z"; when false it must not. The
// returned dateTime is range-validated on success.
func scanDateTime(s string, requireZ bool) (dateTime, error) {
	want := localDateTimeLen
	if requireZ {
		want = utcDateTimeLen
	}
	if len(s) != want {
		return dateTime{}, errBadLength
	}

	// Positional separators: YYYY-MM-DDTHH:MM:SS.
	if s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' || s[16] != ':' {
		return dateTime{}, errBadSeparator
	}
	if requireZ && s[19] != 'Z' {
		return dateTime{}, errMissingZ
	}

	// Each field is a fixed-width, all-digit slice at a known offset.
	var dt dateTime
	for _, f := range [...]struct {
		dst      *int
		from, to int
	}{
		{&dt.year, 0, 4},
		{&dt.month, 5, 7},
		{&dt.day, 8, 10},
		{&dt.hour, 11, 13},
		{&dt.minute, 14, 16},
		{&dt.second, 17, 19},
	} {
		n, err := digits(s[f.from:f.to])
		if err != nil {
			return dateTime{}, err
		}
		*f.dst = n
	}

	if err := dt.validate(); err != nil {
		return dateTime{}, err
	}
	return dt, nil
}

// digits parses an all-ASCII-digit fixed-width field. It rejects signs,
// whitespace, and any non-digit byte, which strconv.Atoi would otherwise
// tolerate (e.g. a leading "+" or "-").
func digits(s string) (int, error) {
	n := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errNotDigit
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// validate checks every component against its valid range, including the
// calendar-correct day count for the month and year.
func (dt dateTime) validate() error {
	switch {
	case dt.year < 1 || dt.year > 9999:
		return errYearRange
	case dt.month < 1 || dt.month > 12:
		return errMonthRange
	case dt.day < 1 || dt.day > daysInMonth(dt.year, dt.month):
		return errDayRange
	case dt.hour < 0 || dt.hour > 23:
		return errHourRange
	case dt.minute < 0 || dt.minute > 59:
		return errMinuteRange
	case dt.second < 0 || dt.second > 59:
		// RFC 8984 Section 1.4.4 inherits RFC 3339 time-second, but
		// JSCalendar does not carry leap seconds: the grammar's second
		// component is 00..59.
		return errSecondRange
	}
	return nil
}

// daysInMonth returns the number of days in the given month, accounting
// for leap years in February.
func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

// isLeapYear reports whether year is a Gregorian leap year.
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// appendText renders the fields onto buf as "YYYY-MM-DDTHH:MM:SS", with a
// trailing "Z" when withZ is true. It assumes in-range fields; callers
// that need conformance guarantees validate first.
func (dt dateTime) appendText(buf []byte, withZ bool) []byte {
	buf = appendPadded(buf, dt.year, 4)
	buf = append(buf, '-')
	buf = appendPadded(buf, dt.month, 2)
	buf = append(buf, '-')
	buf = appendPadded(buf, dt.day, 2)
	buf = append(buf, 'T')
	buf = appendPadded(buf, dt.hour, 2)
	buf = append(buf, ':')
	buf = appendPadded(buf, dt.minute, 2)
	buf = append(buf, ':')
	buf = appendPadded(buf, dt.second, 2)
	if withZ {
		buf = append(buf, 'Z')
	}
	return buf
}

// format renders the date-time as a plain string.
func (dt dateTime) format(withZ bool) string {
	return string(dt.appendText(make([]byte, 0, utcDateTimeLen), withZ))
}

// quote renders the date-time as a JSON string literal (surrounding
// double quotes included). The components are all ASCII digits and
// punctuation, so no JSON escaping is ever required.
func (dt dateTime) quote(withZ bool) []byte {
	buf := make([]byte, 0, utcDateTimeLen+2)
	buf = append(buf, '"')
	buf = dt.appendText(buf, withZ)
	buf = append(buf, '"')
	return buf
}

// appendPadded appends n as a zero-padded decimal of exactly width
// digits. n is assumed non-negative and to fit in width digits, which
// the range validation upstream guarantees. width must be in 1..4 (the
// widest field is the four-digit year); the scratch array is sized to
// that maximum, so a larger width would panic.
func appendPadded(buf []byte, n, width int) []byte {
	var tmp [4]byte
	for i := width - 1; i >= 0; i-- {
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, tmp[:width]...)
}

// unquoteJSONString decodes a JSON string token into its Go string,
// returning errNotJSONString for any non-string JSON value (number,
// object, null, …). Using json.Unmarshal here keeps the string-escape
// handling identical to the rest of the decoder.
func unquoteJSONString(data []byte) (string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", errNotJSONString
	}
	return s, nil
}
