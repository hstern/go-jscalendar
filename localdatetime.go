// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "fmt"

// LocalDateTime is the JSCalendar "LocalDateTime" value type (RFC 8984,
// Section 1.4.4): a calendar date and wall-clock time written as
// "YYYY-MM-DDTHH:MM:SS", with no UTC offset and no trailing "Z".
//
// A LocalDateTime carries no time zone. Per RFC 8984 Section 1.4.4 the
// zone lives in the sibling "timeZone" property of the enclosing object:
// a value with no "timeZone" is floating (it renders in the viewer's
// zone), and one with a "timeZone" is interpreted in that zone. For this
// reason a LocalDateTime is deliberately not modeled as a Go
// [time.Time], which would always carry a [time.Location]; folding the
// two would lose the floating/zoned distinction the spec requires.
//
// The grammar admits only whole seconds. Fractional seconds, an offset,
// or a trailing "Z" are all rejected on unmarshal. UnmarshalJSON is the
// lenient boundary in spirit, but the grammar itself is strict: the spec
// pins the exact textual form, so there is nothing to be lenient about.
type LocalDateTime struct {
	Year   int // 0001..9999
	Month  int // 1..12
	Day    int // 1..31, valid for Month and Year
	Hour   int // 0..23
	Minute int // 0..59
	Second int // 0..59
}

// dt converts to the shared internal currency type.
func (lt LocalDateTime) dt() dateTime {
	return dateTime{
		year:   lt.Year,
		month:  lt.Month,
		day:    lt.Day,
		hour:   lt.Hour,
		minute: lt.Minute,
		second: lt.Second,
	}
}

// localDateTimeFromDT is the inverse of [LocalDateTime.dt].
func localDateTimeFromDT(dt dateTime) LocalDateTime {
	return LocalDateTime{
		Year:   dt.year,
		Month:  dt.month,
		Day:    dt.day,
		Hour:   dt.hour,
		Minute: dt.minute,
		Second: dt.second,
	}
}

// ParseLocalDateTime parses the textual "YYYY-MM-DDTHH:MM:SS" form
// defined by RFC 8984, Section 1.4.4. It rejects any input carrying a
// trailing "Z" (that is a [UTCDateTime]), a UTC offset, fractional
// seconds, or fields outside their valid range.
func ParseLocalDateTime(s string) (LocalDateTime, error) {
	dt, err := scanDateTime(s, false)
	if err != nil {
		return LocalDateTime{}, fmt.Errorf("jscalendar: parse LocalDateTime %q: %w", s, err)
	}
	return localDateTimeFromDT(dt), nil
}

// String returns the RFC 8984 Section 1.4.4 textual form,
// "YYYY-MM-DDTHH:MM:SS". It does not validate; an out-of-range value
// produces a correspondingly out-of-range string. Use
// [LocalDateTime.MarshalJSON] when conformance matters.
func (lt LocalDateTime) String() string {
	return lt.dt().format(false)
}

// MarshalJSON encodes the value as the RFC 8984 Section 1.4.4 string.
//
// Marshal is the strict boundary: a value whose fields fall outside the
// ranges documented on [LocalDateTime] is rejected rather than emitted
// as non-conformant JSON.
func (lt LocalDateTime) MarshalJSON() ([]byte, error) {
	dt := lt.dt()
	if err := dt.validate(); err != nil {
		return nil, fmt.Errorf("jscalendar: marshal LocalDateTime: %w", err)
	}
	return dt.quote(false), nil
}

// UnmarshalJSON decodes a JSON string in the RFC 8984 Section 1.4.4
// form. A non-string token, or a string that does not match the grammar
// exactly, is an error.
func (lt *LocalDateTime) UnmarshalJSON(data []byte) error {
	s, err := unquoteJSONString(data)
	if err != nil {
		return fmt.Errorf("jscalendar: unmarshal LocalDateTime: %w", err)
	}
	v, err := ParseLocalDateTime(s)
	if err != nil {
		return err
	}
	*lt = v
	return nil
}
