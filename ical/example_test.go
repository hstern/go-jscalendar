// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical_test

import (
	"fmt"
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/ical"
)

// These examples mirror the README's iCalendar conversion snippet. Importing
// this sub-package is what pulls in github.com/emersion/go-ical; the core
// jscalendar package stays standard-library-only, so consumers of the typed
// object model who never need iCalendar interop take on no extra dependency.

// ExampleFromICal converts an iCalendar VEVENT into the JSCalendar object
// model. iCalendar is parsed with go-ical; FromICal then maps each component to
// its concrete jscalendar type — a VEVENT becomes a *jscalendar.Event, with the
// UTC DTSTART splitting into a floating start plus the "Etc/UTC" time zone.
func ExampleFromICal() {
	const text = "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//Example Corp//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:event-1\r\n" +
		"DTSTAMP:20260601T120000Z\r\n" +
		"DTSTART:20260701T130000Z\r\n" +
		"DURATION:PT1H\r\n" +
		"SUMMARY:Launch\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	cal, err := goical.NewDecoder(strings.NewReader(text)).Decode()
	if err != nil {
		panic(err)
	}

	objs, err := ical.FromICal(cal)
	if err != nil {
		panic(err)
	}

	ev := objs[0].(*jscalendar.Event)
	fmt.Printf("%s: %s %s in %s\n", ev.Title, ev.Start, ev.Duration, ev.TimeZone)

	// Output:
	// Launch: 2026-07-01T13:00:00 PT1H in Etc/UTC
}

// ExampleToICal converts a JSCalendar Event back into an iCalendar calendar.
// ToICal is the inverse of FromICal: the start plus its named time zone
// recompose into DTSTART;TZID=, and the duration becomes a DURATION property.
func ExampleToICal() {
	start, err := jscalendar.ParseLocalDateTime("2026-07-01T13:00:00")
	if err != nil {
		panic(err)
	}
	dur, err := jscalendar.ParseDuration("PT1H")
	if err != nil {
		panic(err)
	}
	ev := &jscalendar.Event{
		UID:      "event-1",
		Title:    "Launch",
		Start:    &start,
		Duration: &dur,
		TimeZone: jscalendar.TimeZoneId("America/New_York"),
	}

	cal, err := ical.ToICal(ev)
	if err != nil {
		panic(err)
	}

	vevent := cal.Children[0]
	dtstart := vevent.Props.Get(goical.PropDateTimeStart)
	fmt.Printf("%s\n", vevent.Props.Get(goical.PropSummary).Value)
	fmt.Printf("DTSTART;TZID=%s:%s\n", dtstart.Params.Get(goical.ParamTimezoneID), dtstart.Value)

	// Output:
	// Launch
	// DTSTART;TZID=America/New_York:20260701T130000
}
