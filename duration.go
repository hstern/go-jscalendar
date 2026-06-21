// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

// ErrInvalidDuration is the sentinel wrapped by every parse error for the
// duration value types. Test for it with [errors.Is].
var ErrInvalidDuration = errors.New("jscalendar: invalid duration")

// Duration is the JSCalendar Duration value type (RFC 8984, §1.4.6): a
// non-negative span expressed with the ISO 8601 duration grammar that the
// spec pins down — a leading "P", an optional date part, an optional "T"
// separator before the time part, and the zero value "PT0S".
//
// This is deliberately NOT [time.Duration]: the grammars differ, calendar
// units (years, months) have no fixed length, and formatting a
// time.Duration produces output JSCalendar rejects. The fields are kept
// independent so a value round-trips verbatim — a year is never folded
// into days, nor a day into hours.
//
// The week form (PnW) is mutually exclusive with every other unit: when
// Weeks is non-zero all of Years, Months, Days, and Seconds must be zero.
// Seconds carries the entire sub-day time part (hours, minutes, seconds,
// and the optional fractional second), normalized on output into the
// canonical H/M/S grouping.
//
// The zero Duration formats as "PT0S".
type Duration struct {
	Years  uint64
	Months uint64
	Weeks  uint64
	Days   uint64
	// Hours, Minutes, Seconds together describe the time part. They are
	// populated by the parser in canonical (normalized) form, but the
	// formatter re-normalizes so callers may set them freely.
	Hours   uint64
	Minutes uint64
	Seconds uint64
	// Nanos is the fractional-second remainder, 0..999999999. The spec
	// permits a fraction only on the seconds component.
	Nanos uint32
}

// SignedDuration is the JSCalendar SignedDuration value type (RFC 8984,
// §1.4.7): a [Duration] that additionally permits a leading "-". It is the
// type used by alert offsets, where a trigger fires before (negative) or
// after (positive) its anchor.
//
// A negative zero is not meaningful; the zero SignedDuration formats as
// the unsigned "PT0S".
type SignedDuration struct {
	Duration
	// Negative reports whether the value carries a leading minus sign.
	Negative bool
}

// IsZero reports whether the duration is the zero span (every component
// zero), which formats as "PT0S".
func (d Duration) IsZero() bool {
	return d.Years == 0 && d.Months == 0 && d.Weeks == 0 && d.Days == 0 &&
		d.Hours == 0 && d.Minutes == 0 && d.Seconds == 0 && d.Nanos == 0
}

// String formats the duration in the canonical JSCalendar grammar. The
// time part is normalized (seconds overflow into minutes, minutes into
// hours); zero components are omitted except for the all-zero value, which
// renders as "PT0S". The week form renders as "PnW".
func (d Duration) String() string {
	if d.IsZero() {
		return "PT0S"
	}

	var b strings.Builder
	b.WriteByte('P')

	if d.Weeks != 0 {
		// The week form is exclusive; ignore any other field defensively
		// so a malformed in-memory value still produces grammar-valid text.
		b.WriteString(strconv.FormatUint(d.Weeks, 10))
		b.WriteByte('W')
		return b.String()
	}

	if d.Years != 0 {
		b.WriteString(strconv.FormatUint(d.Years, 10))
		b.WriteByte('Y')
	}
	if d.Months != 0 {
		b.WriteString(strconv.FormatUint(d.Months, 10))
		b.WriteByte('M')
	}
	if d.Days != 0 {
		b.WriteString(strconv.FormatUint(d.Days, 10))
		b.WriteByte('D')
	}

	hours, minutes, seconds, nanos := d.normalizedTime()
	if hours != 0 || minutes != 0 || seconds != 0 || nanos != 0 {
		b.WriteByte('T')
		if hours != 0 {
			b.WriteString(strconv.FormatUint(hours, 10))
			b.WriteByte('H')
		}
		if minutes != 0 {
			b.WriteString(strconv.FormatUint(minutes, 10))
			b.WriteByte('M')
		}
		if seconds != 0 || nanos != 0 {
			writeSeconds(&b, seconds, nanos)
		}
	}

	return b.String()
}

// normalizedTime folds the time part into canonical hours/minutes/seconds,
// carrying seconds into minutes and minutes into hours at base 60. Days and
// the calendar units are untouched — only the sub-day part normalizes.
//
// If the components are so large that the total seconds would overflow a
// uint64, normalization is skipped and the fields are returned verbatim:
// the formatter must never silently misreport magnitude. Such a value
// cannot be produced by [ParseDuration] (which rejects it at parse time);
// it can only arise from fields set directly by a caller.
func (d Duration) normalizedTime() (hours, minutes, seconds uint64, nanos uint32) {
	total, ok := totalSeconds(d.Hours, d.Minutes, d.Seconds)
	if !ok {
		return d.Hours, d.Minutes, d.Seconds, d.Nanos
	}
	hours = total / 3600
	total %= 3600
	minutes = total / 60
	seconds = total % 60
	return hours, minutes, seconds, d.Nanos
}

// totalSeconds folds hours/minutes/seconds into a single second count,
// reporting ok=false if any step overflows uint64. The seconds component of
// the JSCalendar time grammar has no fixed upper bound, so adversarial wire
// input ("PT99999999999999999999H") must be rejected rather than wrapped.
func totalSeconds(hours, minutes, seconds uint64) (total uint64, ok bool) {
	h, ok := checkedMul(hours, 3600)
	if !ok {
		return 0, false
	}
	m, ok := checkedMul(minutes, 60)
	if !ok {
		return 0, false
	}
	total, ok = checkedAdd(h, m)
	if !ok {
		return 0, false
	}
	return checkedAdd(total, seconds)
}

func checkedMul(a, b uint64) (uint64, bool) {
	hi, lo := bits.Mul64(a, b)
	return lo, hi == 0
}

func checkedAdd(a, b uint64) (uint64, bool) {
	sum, carry := bits.Add64(a, b, 0)
	return sum, carry == 0
}

// writeSeconds appends the seconds component, including a fractional part
// when nanos is non-zero, with trailing fractional zeros trimmed.
func writeSeconds(b *strings.Builder, seconds uint64, nanos uint32) {
	b.WriteString(strconv.FormatUint(seconds, 10))
	if nanos != 0 {
		frac := strings.TrimRight(fmt.Sprintf("%09d", nanos), "0")
		b.WriteByte('.')
		b.WriteString(frac)
	}
	b.WriteByte('S')
}

// String formats the signed duration, prefixing "-" for negative values.
// A negative zero renders as the unsigned "PT0S".
func (d SignedDuration) String() string {
	if d.Negative && !d.IsZero() {
		return "-" + d.Duration.String()
	}
	return d.Duration.String()
}

// ParseDuration parses the JSCalendar Duration grammar (RFC 8984, §1.4.6).
// It is lenient about non-canonical-but-valid input (for example "PT60S"
// or "P0D"); the resulting value re-formats to canonical form. A leading
// "-" is rejected — that is the SignedDuration grammar; use
// [ParseSignedDuration]. Every failure wraps [ErrInvalidDuration].
func ParseDuration(s string) (Duration, error) {
	if strings.HasPrefix(s, "-") {
		return Duration{}, fmt.Errorf("%w: %q is signed; use ParseSignedDuration", ErrInvalidDuration, s)
	}
	return parseDuration(s)
}

// ParseSignedDuration parses the JSCalendar SignedDuration grammar (RFC
// 8984, §1.4.7): a Duration optionally prefixed with "-". Every failure
// wraps [ErrInvalidDuration].
func ParseSignedDuration(s string) (SignedDuration, error) {
	negative := false
	rest := s
	if r, ok := strings.CutPrefix(s, "-"); ok {
		negative = true
		rest = r
	}
	d, err := parseDuration(rest)
	if err != nil {
		return SignedDuration{}, err
	}
	return SignedDuration{Duration: d, Negative: negative}, nil
}

// parseDuration implements the unsigned grammar shared by both value types.
//
// Grammar (RFC 8984 §1.4.6, narrowing RFC 3339 Appendix A):
//
//	dur-second = 1*DIGIT "S"
//	dur-minute = 1*DIGIT "M" [dur-second]
//	dur-hour   = 1*DIGIT "H" [dur-minute]
//	dur-time   = "T" (dur-hour / dur-minute / dur-second)
//	dur-day    = 1*DIGIT "D"
//	dur-week   = 1*DIGIT "W"
//	dur-month  = 1*DIGIT "M" [dur-day]
//	dur-year   = 1*DIGIT "Y" [dur-month]
//	dur-date   = (dur-day / dur-month / dur-year) [dur-time]
//	duration   = "P" (dur-date / dur-time / dur-week)
//
// JSCalendar additionally permits a fractional second on the seconds
// component. There is no bare "P": at least one component is required.
func parseDuration(s string) (Duration, error) {
	body, ok := strings.CutPrefix(s, "P")
	if !ok {
		return Duration{}, fmt.Errorf("%w: %q does not start with P", ErrInvalidDuration, s)
	}
	if body == "" {
		return Duration{}, fmt.Errorf("%w: %q has no components", ErrInvalidDuration, s)
	}

	date, timePart, hasT := strings.Cut(body, "T")

	var d Duration

	// The week form occupies the whole date part and forbids a time part.
	weeks, isWeek, err := parseWeekForm(date)
	if err != nil {
		return Duration{}, wrapDur(err, s)
	}
	if isWeek {
		if hasT {
			return Duration{}, fmt.Errorf("%w: %q combines weeks with a time part", ErrInvalidDuration, s)
		}
		d.Weeks = weeks
		return d, nil
	}

	if err := parseDatePart(date, &d); err != nil {
		return Duration{}, wrapDur(err, s)
	}

	if hasT {
		if timePart == "" {
			return Duration{}, fmt.Errorf("%w: %q has a T with no time component", ErrInvalidDuration, s)
		}
		if err := parseTimePart(timePart, &d); err != nil {
			return Duration{}, wrapDur(err, s)
		}
	}

	return d, nil
}

// parseWeekForm recognizes a bare "nW" date part. It returns ok=false when
// the part is not a week form (so the caller falls through to the calendar
// date parser) and an error only when it looks like a week form but is
// malformed.
func parseWeekForm(date string) (weeks uint64, ok bool, err error) {
	if !strings.HasSuffix(date, "W") {
		return 0, false, nil
	}
	digits := strings.TrimSuffix(date, "W")
	n, err := parseUint(digits)
	if err != nil {
		return 0, false, fmt.Errorf("week component: %w", err)
	}
	return n, true, nil
}

// dateUnit pairs a date-component suffix with the field it fills, in the
// order the grammar requires them to appear.
type dateUnit struct {
	suffix byte
	set    func(d *Duration, v uint64)
}

var dateUnits = []dateUnit{
	{'Y', func(d *Duration, v uint64) { d.Years = v }},
	{'M', func(d *Duration, v uint64) { d.Months = v }},
	{'D', func(d *Duration, v uint64) { d.Days = v }},
}

// timeUnit pairs a time-component suffix with its field, in grammar order.
type timeUnit struct {
	suffix byte
	frac   bool // only the seconds component may carry a fraction
	set    func(d *Duration, whole uint64, nanos uint32)
}

var timeUnits = []timeUnit{
	{'H', false, func(d *Duration, v uint64, _ uint32) { d.Hours = v }},
	{'M', false, func(d *Duration, v uint64, _ uint32) { d.Minutes = v }},
	{'S', true, func(d *Duration, v uint64, n uint32) { d.Seconds = v; d.Nanos = n }},
}

// parseDatePart consumes the Y/M/D components in order. An empty part is
// valid only when a time part follows; the caller enforces that.
func parseDatePart(date string, d *Duration) error {
	rest := date
	idx := 0
	for rest != "" {
		whole, _, suffix, after, err := scanComponent(rest, false)
		if err != nil {
			return err
		}
		matched := false
		for ; idx < len(dateUnits); idx++ {
			if dateUnits[idx].suffix == suffix {
				dateUnits[idx].set(d, whole)
				idx++
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("unexpected or out-of-order date unit %q", string(suffix))
		}
		rest = after
	}
	return nil
}

// parseTimePart consumes the H/M/S components in order.
func parseTimePart(timePart string, d *Duration) error {
	rest := timePart
	idx := 0
	for rest != "" {
		whole, nanos, suffix, after, err := scanComponent(rest, true)
		if err != nil {
			return err
		}
		matched := false
		for ; idx < len(timeUnits); idx++ {
			if timeUnits[idx].suffix == suffix {
				if nanos != 0 && !timeUnits[idx].frac {
					return fmt.Errorf("fractional value not allowed on %q component", string(suffix))
				}
				timeUnits[idx].set(d, whole, nanos)
				idx++
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("unexpected or out-of-order time unit %q", string(suffix))
		}
		rest = after
	}
	if _, ok := totalSeconds(d.Hours, d.Minutes, d.Seconds); !ok {
		return errors.New("time part overflows the representable range")
	}
	return nil
}

// scanComponent reads one "1*DIGIT [ '.' 1*DIGIT ] UNIT" component from the
// front of s. When allowFrac is false a fractional point is an error.
func scanComponent(s string, allowFrac bool) (whole uint64, nanos uint32, suffix byte, after string, err error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, 0, "", fmt.Errorf("expected a digit at %q", s)
	}
	intPart := s[:i]

	var fracPart string
	if i < len(s) && s[i] == '.' {
		if !allowFrac {
			return 0, 0, 0, "", fmt.Errorf("fraction not allowed in %q", s)
		}
		i++
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return 0, 0, 0, "", fmt.Errorf("empty fraction in %q", s)
		}
		fracPart = s[start:i]
	}

	if i >= len(s) {
		return 0, 0, 0, "", fmt.Errorf("number %q with no unit", s)
	}
	suffix = s[i]
	after = s[i+1:]

	whole, err = parseUint(intPart)
	if err != nil {
		return 0, 0, 0, "", err
	}
	if fracPart != "" {
		nanos, err = fractionToNanos(fracPart)
		if err != nil {
			return 0, 0, 0, "", err
		}
	}
	return whole, nanos, suffix, after, nil
}

// fractionToNanos converts a decimal fraction string (the digits after the
// point, without it) into nanoseconds, truncating beyond nanosecond
// resolution.
func fractionToNanos(frac string) (uint32, error) {
	if len(frac) > 9 {
		frac = frac[:9]
	}
	padded := frac + strings.Repeat("0", 9-len(frac))
	n, err := strconv.ParseUint(padded, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("fraction %q: %w", frac, err)
	}
	return uint32(n), nil
}

// parseUint parses a base-10 component magnitude, rejecting empty input and
// leading-sign characters (the grammar forbids signs inside a component).
func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("empty number")
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("number %q: %w", s, err)
	}
	return n, nil
}

func wrapDur(err error, s string) error {
	return fmt.Errorf("%w: %q: %s", ErrInvalidDuration, s, err.Error())
}

// MarshalText implements [encoding.TextMarshaler], emitting the canonical
// grammar form.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler] using the lenient
// [ParseDuration] grammar.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalText implements [encoding.TextMarshaler] for SignedDuration.
func (d SignedDuration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler] for SignedDuration.
func (d *SignedDuration) UnmarshalText(text []byte) error {
	parsed, err := ParseSignedDuration(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
