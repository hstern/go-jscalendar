// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RecurrenceRule is the JSCalendar "RecurrenceRule" value type (RFC 8984,
// Section 4.3.1): the structured, JSON-native replacement for the opaque
// iCalendar "RRULE" string. A rule describes how a set of recurrence
// instances is generated from a base date-time (the enclosing object's
// "start"), as a frequency that is optionally narrowed by a collection of
// "by*" filters and bounded by either a "count" or an "until".
//
// Only [RecurrenceRule.Frequency] is mandatory; every other field is
// optional and, when absent, carries the default the spec assigns it
// (documented per field below). The zero value is therefore not a valid
// rule on its own — Frequency must be set before marshaling — but it is a
// convenient starting point for building one.
//
// # Lenient unmarshal, strict marshal
//
// Following the library's Postel's-law posture, [RecurrenceRule.UnmarshalJSON]
// accepts any JSON object and never rejects a rule for semantic reasons: an
// out-of-range "byHour", a "frequency" the spec does not define, or the
// presence of both "count" and "until" all decode without error so the rule
// round-trips. The two cross-field constraints the spec imposes are checked
// only at the opt-in validation boundary of a later phase, not here:
//
//   - "frequency" is REQUIRED (Section 4.3.1). A rule with no Frequency is
//     malformed; [RecurrenceRule.MarshalJSON] rejects it, but unmarshal
//     tolerates its absence so a partial document round-trips.
//   - "count" and "until" are MUTUALLY EXCLUSIVE (Section 4.3.1): a rule
//     bounds its instances by one or the other, never both. This package
//     does not reject a rule that sets both; the conflict is a validation
//     concern. [RecurrenceRule.HasCount] and [RecurrenceRule.HasUntil]
//     expose which bounds are present so a validator can flag the overlap.
//
// # Until is a LocalDateTime, not UTC
//
// [RecurrenceRule.Until] is a [LocalDateTime] (Section 4.3.1), interpreted
// in the time zone of the enclosing object — the same "timeZone" that
// governs the object's "start" — and NOT in UTC. An "until" of
// "2020-01-31T23:59:59" therefore means the last instant of January 31st in
// the event's own zone, which is a different absolute instant for an event
// in "America/New_York" than for one in "Europe/Berlin". Folding it into a
// UTC instant would silently shift the recurrence boundary; the value is
// kept zone-free for exactly the reason [LocalDateTime] is.
type RecurrenceRule struct {
	// Frequency is the base period of the recurrence (REQUIRED): one of
	// "yearly", "monthly", "weekly", "daily", "hourly", "minutely", or
	// "secondly" (Section 4.3.1). It has no default; a rule without a
	// Frequency is malformed.
	Frequency Frequency

	// Interval is the multiplier applied to Frequency: an Interval of 2 with
	// a "weekly" Frequency recurs every other week (Section 4.3.1). The spec
	// requires Interval >= 1 and defaults it to 1 when absent. A zero
	// Interval is treated as "absent" on marshal (the default 1 is emitted
	// by omission); use a value of 1 or more to pin a specific interval.
	Interval uint

	// RScale names the calendar system the rule is evaluated in — a value
	// from the CALENDARS registry of RFC 7529, such as "gregorian" or
	// "chinese" (Section 4.3.1). It defaults to "gregorian" when absent and
	// is held lowercase as it appears on the wire.
	RScale string

	// Skip selects how an instance that lands on a date absent from RScale's
	// calendar (a leap-month day in a non-leap year, February 30th) is
	// handled: "omit" (drop it), "backward", or "forward" (Section 4.3.1).
	// It defaults to "omit" when absent.
	Skip string

	// FirstDayOfWeek is the weekday a week begins on for the purpose of
	// "weekly" expansion and the "byWeekNo" filter: one of the two-letter
	// codes "mo", "tu", "we", "th", "fr", "sa", "su" (Section 4.3.1). It
	// defaults to "mo" when absent.
	FirstDayOfWeek string

	// ByDay narrows the recurrence to particular days of the week, each
	// optionally pinned to a specific occurrence within the period — for
	// example the last Sunday of the month (Section 4.3.1). See [NDay].
	ByDay []NDay

	// ByMonthDay narrows the recurrence to particular days of the month,
	// counted from the start (1..31) or, with a negative value, from the end
	// (-1 is the last day; Section 4.3.1). Zero is not a valid entry.
	ByMonthDay []int

	// ByMonth narrows the recurrence to particular months. Its values are
	// STRINGS, not integers: "1" through "12", each optionally carrying a
	// trailing "L" to name the leap month of that ordinal in a calendar that
	// has one (Section 4.3.1). The string form is what lets a leap month be
	// expressed at all, which is why the wire type is not a plain integer.
	ByMonth []string

	// ByYearDay narrows the recurrence to particular days of the year,
	// counted from the start (1..366) or, with a negative value, from the
	// end (-1 is the last day; Section 4.3.1). Zero is not a valid entry.
	ByYearDay []int

	// ByWeekNo narrows the recurrence to particular weeks of the year,
	// counted from the start (1..53) or, with a negative value, from the end
	// (-1 is the last week; Section 4.3.1). Week boundaries follow
	// FirstDayOfWeek. Zero is not a valid entry.
	ByWeekNo []int

	// ByHour narrows the recurrence to particular hours of the day, 0..23
	// (Section 4.3.1).
	ByHour []uint

	// ByMinute narrows the recurrence to particular minutes of the hour,
	// 0..59 (Section 4.3.1).
	ByMinute []uint

	// BySecond narrows the recurrence to particular seconds of the minute,
	// 0..60 — 60 admits a leap second (Section 4.3.1).
	BySecond []uint

	// BySetPosition selects particular instances out of the set produced by
	// the other filters within each Frequency period: 1 is the first, -1 the
	// last (Section 4.3.1). It is applied after every "by*" filter above.
	BySetPosition []int

	// Count bounds the recurrence to a fixed number of instances, inclusive
	// of the first (Section 4.3.1). It is mutually exclusive with Until; a
	// rule sets at most one. Count is a pointer so that "no bound" (nil) is
	// distinguishable from an explicit zero, and so the field can be omitted
	// from the marshaled object when unset. See [RecurrenceRule.HasCount].
	Count *uint

	// Until bounds the recurrence to instances no later than this
	// [LocalDateTime], interpreted in the enclosing object's time zone, not
	// UTC (Section 4.3.1). It is mutually exclusive with Count. Until is a
	// pointer so that "no bound" (nil) is distinguishable from the zero
	// LocalDateTime, and so the field can be omitted when unset. See
	// [RecurrenceRule.HasUntil].
	Until *LocalDateTime
}

// Frequency is the base period of a [RecurrenceRule] (RFC 8984,
// Section 4.3.1). The constants below enumerate the seven values the spec
// defines; the type is an open string so an unrecognized frequency decoded
// from the wire round-trips rather than being rejected.
type Frequency string

// The frequencies defined by RFC 8984, Section 4.3.1, from the longest base
// period to the shortest.
const (
	FrequencyYearly   Frequency = "yearly"
	FrequencyMonthly  Frequency = "monthly"
	FrequencyWeekly   Frequency = "weekly"
	FrequencyDaily    Frequency = "daily"
	FrequencyHourly   Frequency = "hourly"
	FrequencyMinutely Frequency = "minutely"
	FrequencySecondly Frequency = "secondly"
)

// IsValid reports whether f is one of the seven frequencies defined by RFC
// 8984, Section 4.3.1. It is opt-in: decoding does not call it, so a rule
// carrying an unrecognized frequency still round-trips unchanged.
func (f Frequency) IsValid() bool {
	switch f {
	case FrequencyYearly, FrequencyMonthly, FrequencyWeekly, FrequencyDaily,
		FrequencyHourly, FrequencyMinutely, FrequencySecondly:
		return true
	default:
		return false
	}
}

// HasCount reports whether the rule is bounded by an instance count — that
// is, whether [RecurrenceRule.Count] is set. A rule has at most one of
// HasCount and [RecurrenceRule.HasUntil] true; a validator uses the pair to
// flag a rule that sets both, which Section 4.3.1 forbids.
func (r RecurrenceRule) HasCount() bool { return r.Count != nil }

// HasUntil reports whether the rule is bounded by an end date-time — that
// is, whether [RecurrenceRule.Until] is set. See [RecurrenceRule.HasCount]
// for the mutual-exclusion rule.
func (r RecurrenceRule) HasUntil() bool { return r.Until != nil }

// recurrenceRuleType is the value of the "@type" member that RFC 8984,
// Section 4.3.1, assigns to a RecurrenceRule. It is emitted first so the
// marshaled object is byte-stable and self-describing.
const recurrenceRuleType = "RecurrenceRule"

// recurrenceRuleJSON is the wire mirror of [RecurrenceRule]. Every field is
// pointer- or slice- or omitempty-typed so that a default-valued or absent
// member is omitted from the marshaled object, matching the spec's "absent
// means the documented default" rule and keeping the output compact and
// byte-stable.
//
// The "@type" member is encoded explicitly and first by [RecurrenceRule.MarshalJSON];
// it is not a field here so that decoding tolerates its absence or any
// casing without a struct-tag mismatch.
type recurrenceRuleJSON struct {
	Frequency      Frequency      `json:"frequency"`
	Interval       uint           `json:"interval,omitempty"`
	RScale         string         `json:"rscale,omitempty"`
	Skip           string         `json:"skip,omitempty"`
	FirstDayOfWeek string         `json:"firstDayOfWeek,omitempty"`
	ByDay          []NDay         `json:"byDay,omitempty"`
	ByMonthDay     []int          `json:"byMonthDay,omitempty"`
	ByMonth        []string       `json:"byMonth,omitempty"`
	ByYearDay      []int          `json:"byYearDay,omitempty"`
	ByWeekNo       []int          `json:"byWeekNo,omitempty"`
	ByHour         []uint         `json:"byHour,omitempty"`
	ByMinute       []uint         `json:"byMinute,omitempty"`
	BySecond       []uint         `json:"bySecond,omitempty"`
	BySetPosition  []int          `json:"bySetPosition,omitempty"`
	Count          *uint          `json:"count,omitempty"`
	Until          *LocalDateTime `json:"until,omitempty"`
}

// MarshalJSON encodes the rule as a JSON object with the "@type" member
// emitted first, per RFC 8984, Section 4.3.1.
//
// Marshal is the strict boundary for the one structural requirement this
// type can check on its own: a rule with no [RecurrenceRule.Frequency] is
// rejected rather than emitted as a non-conformant object missing its
// mandatory member. The cross-field "count" / "until" exclusivity is a
// validation concern (see the type doc) and is not enforced here.
//
// Default-valued optional members (Interval 0, empty strings, nil slices and
// pointers) are omitted, so a minimal rule marshals to just its "@type" and
// "frequency".
func (r RecurrenceRule) MarshalJSON() ([]byte, error) {
	if r.Frequency == "" {
		return nil, fmt.Errorf("jscalendar: marshal RecurrenceRule: frequency is required")
	}

	// recurrenceRuleJSON has the same fields in the same order as
	// RecurrenceRule, so a direct conversion attaches the struct tags
	// without restating every field.
	body, err := json.Marshal(recurrenceRuleJSON(r))
	if err != nil {
		return nil, fmt.Errorf("jscalendar: marshal RecurrenceRule: %w", err)
	}

	return prependType(recurrenceRuleType, body), nil
}

// UnmarshalJSON decodes a JSON object into the rule. It is lenient: any JSON
// object is accepted, an "@type" member (if present) is ignored rather than
// required to equal "RecurrenceRule", and no range or cross-field check is
// performed — an out-of-range filter value or a rule carrying both "count"
// and "until" decodes without error so the rule round-trips. A non-object
// input — array, scalar, or null — is an error, since a RecurrenceRule is
// always an object.
func (r *RecurrenceRule) UnmarshalJSON(data []byte) error {
	if isJSONNull(data) {
		return fmt.Errorf("jscalendar: unmarshal RecurrenceRule: unexpected null")
	}

	var aux recurrenceRuleJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("jscalendar: unmarshal RecurrenceRule: %w", err)
	}

	*r = RecurrenceRule(aux)
	return nil
}

// NDay is the JSCalendar "NDay" value type (RFC 8984, Section 4.3.1): an
// entry in a [RecurrenceRule]'s ByDay filter naming a weekday and,
// optionally, which occurrence of that weekday within the recurrence period
// it pins.
//
// "Every Tuesday" is an NDay with Day "tu" and no NthOfPeriod. "The last
// Sunday of the month" is an NDay with Day "su" and NthOfPeriod -1, used in
// a monthly rule. A positive NthOfPeriod counts from the start of the period
// (1 is the first such weekday), a negative one from the end (-1 is the
// last); the occurrence number is meaningful only against the enclosing
// rule's Frequency, so the same NDay means "first Monday of the month" in a
// monthly rule and "first Monday of the year" in a yearly one.
type NDay struct {
	// Day is the weekday this entry selects: one of the two-letter codes
	// "mo", "tu", "we", "th", "fr", "sa", "su" (REQUIRED; Section 4.3.1).
	Day string

	// NthOfPeriod pins the entry to a specific occurrence of Day within the
	// recurrence period: 1 is the first, -1 the last (Section 4.3.1). It is a
	// pointer so that "every Day" (nil) is distinguishable from an explicit
	// occurrence, and so the member is omitted from the marshaled object when
	// unset.
	NthOfPeriod *int
}

// nDayType is the value of the "@type" member that RFC 8984, Section 4.3.1,
// assigns to an NDay.
const nDayType = "NDay"

// nDayJSON is the wire mirror of [NDay]. NthOfPeriod is omitted when nil so
// the common "every weekday" entry marshals to just its "@type" and "day".
type nDayJSON struct {
	Day         string `json:"day"`
	NthOfPeriod *int   `json:"nthOfPeriod,omitempty"`
}

// MarshalJSON encodes the entry as a JSON object with "@type" first, per RFC
// 8984, Section 4.3.1.
//
// Marshal is the strict boundary for the one member this type can check: an
// NDay with an empty [NDay.Day] is rejected rather than emitted as an object
// missing its mandatory "day".
func (n NDay) MarshalJSON() ([]byte, error) {
	if n.Day == "" {
		return nil, fmt.Errorf("jscalendar: marshal NDay: day is required")
	}

	body, err := json.Marshal(nDayJSON(n))
	if err != nil {
		return nil, fmt.Errorf("jscalendar: marshal NDay: %w", err)
	}

	return prependType(nDayType, body), nil
}

// UnmarshalJSON decodes a JSON object into the entry. It is lenient: any JSON
// object is accepted, an "@type" member (if present) is ignored, and an
// unrecognized "day" code is not rejected so the entry round-trips. A
// non-object input is an error.
func (n *NDay) UnmarshalJSON(data []byte) error {
	if isJSONNull(data) {
		return fmt.Errorf("jscalendar: unmarshal NDay: unexpected null")
	}

	var aux nDayJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("jscalendar: unmarshal NDay: %w", err)
	}

	*n = NDay(aux)
	return nil
}

// prependType inserts a leading "@type" member with the given value into the
// JSON object encoded in body, returning a new object whose first member is
// "@type". RFC 8984 requires "@type" on every object and the library emits
// it first for byte-stable, self-describing output (Section 1.4.1).
//
// body must be a non-empty JSON object ("{...}"); the sole caller is a
// MarshalJSON method that produced it via [json.Marshal] of a struct, so this
// holds by construction. An empty object "{}" yields {"@type":"<typ>"}.
func prependType(typ string, body []byte) []byte {
	// `{"@type":"<typ>"` — the open brace and the type member.
	prefix := fmt.Sprintf(`{"@type":%q`, typ)

	var buf bytes.Buffer
	buf.Grow(len(prefix) + len(body))
	buf.WriteString(prefix)
	// body is "{...}". Drop its leading "{"; if it carried any members, the
	// next byte is '"', so a comma separates "@type" from them. An empty
	// "{}" leaves just the closing "}".
	rest := body[1:]
	if len(rest) > 1 {
		buf.WriteByte(',')
	}
	buf.Write(rest)
	return buf.Bytes()
}
