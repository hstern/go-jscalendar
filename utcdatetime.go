// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"fmt"
	"time"
)

// UTCDateTime is the JSCalendar "UTCDateTime" value type (RFC 8984,
// Section 1.4.5): the same "YYYY-MM-DDTHH:MM:SS" grammar as
// [LocalDateTime] but with a mandatory trailing "Z", denoting an instant
// in UTC.
//
// Unlike [LocalDateTime], a UTCDateTime denotes an absolute instant, so
// it can convert losslessly to and from a UTC [time.Time] via
// [UTCDateTime.Time] and [UTCDateTimeFromTime]. The grammar admits only
// whole seconds; fractional seconds, an offset other than the literal
// "Z", or a missing "Z" are all rejected on unmarshal.
type UTCDateTime struct {
	Year   int // 0001..9999
	Month  int // 1..12
	Day    int // 1..31, valid for Month and Year
	Hour   int // 0..23
	Minute int // 0..59
	Second int // 0..59
}

// dt converts to the shared internal currency type.
func (ut UTCDateTime) dt() dateTime {
	return dateTime{
		year:   ut.Year,
		month:  ut.Month,
		day:    ut.Day,
		hour:   ut.Hour,
		minute: ut.Minute,
		second: ut.Second,
	}
}

// utcDateTimeFromDT is the inverse of [UTCDateTime.dt].
func utcDateTimeFromDT(dt dateTime) UTCDateTime {
	return UTCDateTime{
		Year:   dt.year,
		Month:  dt.month,
		Day:    dt.day,
		Hour:   dt.hour,
		Minute: dt.minute,
		Second: dt.second,
	}
}

// ParseUTCDateTime parses the textual "YYYY-MM-DDTHH:MM:SSZ" form
// defined by RFC 8984, Section 1.4.5. It rejects any input that omits
// the trailing "Z" (that is a [LocalDateTime]), carries a numeric
// offset, has fractional seconds, or has fields outside their valid
// range.
func ParseUTCDateTime(s string) (UTCDateTime, error) {
	dt, err := scanDateTime(s, true)
	if err != nil {
		return UTCDateTime{}, fmt.Errorf("jscalendar: parse UTCDateTime %q: %w", s, err)
	}
	return utcDateTimeFromDT(dt), nil
}

// UTCDateTimeFromTime converts a [time.Time] to a UTCDateTime. The
// instant is first normalized to UTC, so the wall-clock fields reflect
// the UTC representation regardless of the input's location. Sub-second
// precision is truncated, since the wire grammar is whole-second.
func UTCDateTimeFromTime(t time.Time) UTCDateTime {
	t = t.UTC()
	return UTCDateTime{
		Year:   t.Year(),
		Month:  int(t.Month()),
		Day:    t.Day(),
		Hour:   t.Hour(),
		Minute: t.Minute(),
		Second: t.Second(),
	}
}

// Time returns the instant as a UTC [time.Time]. It does not validate;
// out-of-range fields are normalized by [time.Date] the same way the
// standard library normalizes them.
func (ut UTCDateTime) Time() time.Time {
	return time.Date(ut.Year, time.Month(ut.Month), ut.Day, ut.Hour, ut.Minute, ut.Second, 0, time.UTC)
}

// String returns the RFC 8984 Section 1.4.5 textual form,
// "YYYY-MM-DDTHH:MM:SSZ". It does not validate; use
// [UTCDateTime.MarshalJSON] when conformance matters.
func (ut UTCDateTime) String() string {
	return ut.dt().format(true)
}

// MarshalJSON encodes the value as the RFC 8984 Section 1.4.5 string,
// including the trailing "Z".
//
// Marshal is the strict boundary: a value whose fields fall outside the
// ranges documented on [UTCDateTime] is rejected rather than emitted as
// non-conformant JSON.
func (ut UTCDateTime) MarshalJSON() ([]byte, error) {
	dt := ut.dt()
	if err := dt.validate(); err != nil {
		return nil, fmt.Errorf("jscalendar: marshal UTCDateTime: %w", err)
	}
	return dt.quote(true), nil
}

// UnmarshalJSON decodes a JSON string in the RFC 8984 Section 1.4.5
// form. A non-string token, or a string that does not match the grammar
// exactly (including the mandatory trailing "Z"), is an error.
func (ut *UTCDateTime) UnmarshalJSON(data []byte) error {
	s, err := unquoteJSONString(data)
	if err != nil {
		return fmt.Errorf("jscalendar: unmarshal UTCDateTime: %w", err)
	}
	v, err := ParseUTCDateTime(s)
	if err != nil {
		return err
	}
	*ut = v
	return nil
}
