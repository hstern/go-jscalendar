// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jmap_test

import (
	"encoding/json"
	"fmt"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/jmap"
)

// ExampleFromEvent shows wrapping a bare JSCalendar Event as a JMAP
// CalendarEvent, assigning it to a Calendar, and marshaling it to the wire
// form a CalendarEvent/set create would carry.
func ExampleFromEvent() {
	start, _ := jscalendar.ParseLocalDateTime("2026-06-21T14:00:00")
	dur, _ := jscalendar.ParseDuration("PT1H")

	ce := jmap.FromEvent(&jscalendar.Event{
		UID:      "a1b2c3",
		Title:    "Sprint review",
		Start:    &start,
		Duration: &dur,
		TimeZone: "America/Toronto",
	})
	ce.CalendarIDs = map[jscalendar.Id]bool{"work": true}
	ce.IsDraft = true

	out, _ := json.Marshal(ce)
	fmt.Println(string(out))
	// Output: {"@type":"Event","uid":"a1b2c3","title":"Sprint review","start":"2026-06-21T14:00:00","duration":"PT1H","timeZone":"America/Toronto","calendarIds":{"work":true},"isDraft":true}
}

// ExampleCalendarEvent_ToEvent shows decoding a CalendarEvent off the wire and
// unwrapping it to the plain JSCalendar Event for code that speaks RFC 8984.
func ExampleCalendarEvent_ToEvent() {
	wire := `{"@type":"Event","uid":"a1b2c3","title":"Sprint review",` +
		`"calendarIds":{"work":true},"isDraft":true}`

	var ce jmap.CalendarEvent
	if err := json.Unmarshal([]byte(wire), &ce); err != nil {
		panic(err)
	}

	ev := ce.ToEvent()
	fmt.Printf("title=%q draft=%t calendars=%v\n", ev.Title, ce.IsDraft, ce.CalendarIDs)
	// Output: title="Sprint review" draft=true calendars=map[work:true]
}
