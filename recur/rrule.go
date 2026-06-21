// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	jscalendar "github.com/hstern/go-jscalendar"
	"github.com/teambition/rrule-go"
)

// frequencies maps the seven JSCalendar "frequency" values (RFC 8984,
// Section 4.3.1) to the rrule-go [rrule.Frequency] codes.
var frequencies = map[jscalendar.Frequency]rrule.Frequency{
	jscalendar.FrequencyYearly:   rrule.YEARLY,
	jscalendar.FrequencyMonthly:  rrule.MONTHLY,
	jscalendar.FrequencyWeekly:   rrule.WEEKLY,
	jscalendar.FrequencyDaily:    rrule.DAILY,
	jscalendar.FrequencyHourly:   rrule.HOURLY,
	jscalendar.FrequencyMinutely: rrule.MINUTELY,
	jscalendar.FrequencySecondly: rrule.SECONDLY,
}

// weekdays maps the two-letter JSCalendar weekday codes (RFC 8984,
// Section 4.3.1) to the rrule-go [rrule.Weekday] values.
var weekdays = map[string]rrule.Weekday{
	"mo": rrule.MO,
	"tu": rrule.TU,
	"we": rrule.WE,
	"th": rrule.TH,
	"fr": rrule.FR,
	"sa": rrule.SA,
	"su": rrule.SU,
}

// buildRRule translates one JSCalendar [jscalendar.RecurrenceRule] into an
// rrule-go [*rrule.RRule] anchored at dtstart (the master start resolved in its
// time zone). It returns an error for the parts of Section 4.3.1 this first cut
// does not implement — a non-Gregorian "rscale" and the leap-month "byMonth"
// form — rather than silently producing wrong instances.
func buildRRule(r jscalendar.RecurrenceRule, dtstart time.Time) (*rrule.RRule, error) {
	opt, err := buildROption(r, dtstart)
	if err != nil {
		return nil, err
	}
	rr, err := rrule.NewRRule(opt)
	if err != nil {
		return nil, fmt.Errorf("recur: build recurrence rule: %w", err)
	}
	return rr, nil
}

// buildROption assembles the [rrule.ROption] for a rule. It is split out from
// buildRRule so the translation can be unit-tested without constructing the
// iterator.
func buildROption(r jscalendar.RecurrenceRule, dtstart time.Time) (rrule.ROption, error) {
	if r.RScale != "" && !strings.EqualFold(r.RScale, "gregorian") {
		return rrule.ROption{}, fmt.Errorf(
			"recur: rscale %q is not supported; only the Gregorian calendar is implemented", r.RScale,
		)
	}

	freq, ok := frequencies[r.Frequency]
	if !ok {
		return rrule.ROption{}, fmt.Errorf("recur: unknown frequency %q", r.Frequency)
	}

	opt := rrule.ROption{
		Freq:    freq,
		Dtstart: dtstart,
	}

	if r.Interval > 0 {
		opt.Interval = int(r.Interval)
	}
	if r.Count != nil {
		opt.Count = int(*r.Count)
	}
	if r.Until != nil {
		// "until" is a LocalDateTime in the master's zone, the same zone
		// dtstart carries (Section 4.3.1); resolve it there, not in UTC.
		opt.Until = localToTime(*r.Until, dtstart.Location())
	}
	if r.FirstDayOfWeek != "" {
		wd, ok := weekdays[strings.ToLower(r.FirstDayOfWeek)]
		if !ok {
			return rrule.ROption{}, fmt.Errorf("recur: unknown firstDayOfWeek %q", r.FirstDayOfWeek)
		}
		opt.Wkst = wd
	}

	if err := applyByDay(&opt, r.ByDay); err != nil {
		return rrule.ROption{}, err
	}
	if err := applyByMonth(&opt, r.ByMonth); err != nil {
		return rrule.ROption{}, err
	}

	opt.Bymonthday = r.ByMonthDay
	opt.Byyearday = r.ByYearDay
	opt.Byweekno = r.ByWeekNo
	opt.Bysetpos = r.BySetPosition
	opt.Byhour = uintsToInts(r.ByHour)
	opt.Byminute = uintsToInts(r.ByMinute)
	opt.Bysecond = uintsToInts(r.BySecond)

	return opt, nil
}

// applyByDay translates the JSCalendar NDay list (weekday plus optional
// positional "nthOfPeriod") into rrule-go's positional weekday form.
func applyByDay(opt *rrule.ROption, byDay []jscalendar.NDay) error {
	if len(byDay) == 0 {
		return nil
	}
	out := make([]rrule.Weekday, 0, len(byDay))
	for _, nd := range byDay {
		wd, ok := weekdays[strings.ToLower(nd.Day)]
		if !ok {
			return fmt.Errorf("recur: unknown byDay weekday %q", nd.Day)
		}
		if nd.NthOfPeriod != nil {
			wd = wd.Nth(*nd.NthOfPeriod)
		}
		out = append(out, wd)
	}
	opt.Byweekday = out
	return nil
}

// applyByMonth translates the JSCalendar string "byMonth" values into the
// integer month numbers rrule-go expects. A leap-month value (a trailing "L",
// Section 4.3.1) is rejected: it is only meaningful under a non-Gregorian
// rscale, which this cut does not support.
func applyByMonth(opt *rrule.ROption, byMonth []string) error {
	if len(byMonth) == 0 {
		return nil
	}
	out := make([]int, 0, len(byMonth))
	for _, m := range byMonth {
		if strings.HasSuffix(m, "L") || strings.HasSuffix(m, "l") {
			return fmt.Errorf("recur: leap-month byMonth value %q is not supported", m)
		}
		n, err := strconv.Atoi(m)
		if err != nil || n < 1 || n > 12 {
			return fmt.Errorf("recur: invalid byMonth value %q", m)
		}
		out = append(out, n)
	}
	opt.Bymonth = out
	return nil
}

// uintsToInts widens a slice of unsigned by* values to the int slice rrule-go
// takes. The JSCalendar ranges (hour 0..23, minute/second 0..60) fit an int on
// every supported platform.
func uintsToInts(in []uint) []int {
	if len(in) == 0 {
		return nil
	}
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
