// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur_test

import (
	"encoding/json"
	"fmt"
	"time"

	jscalendar "github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/recur"
)

// ExampleOccurrences expands a weekly event into its concrete instances over a
// one-month window. Each instance carries the recurrence id that identifies it
// within the series.
func ExampleOccurrences() {
	start, _ := jscalendar.ParseLocalDateTime("2020-01-01T09:00:00")
	count := uint(3)
	e := &jscalendar.Event{
		Type: "Event", UID: "weekly-sync",
		Title:    "Weekly sync",
		Start:    &start,
		TimeZone: "America/New_York",
		RecurrenceRules: []jscalendar.RecurrenceRule{
			{Frequency: jscalendar.FrequencyWeekly, Count: &count},
		},
	}

	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)

	occurrences, err := recur.Occurrences(e, from, until)
	if err != nil {
		panic(err)
	}
	for _, o := range occurrences {
		fmt.Println(o.RecurrenceID, o.RecurrenceIDTimeZone)
	}
	// Output:
	// 2020-01-01T09:00:00 America/New_York
	// 2020-01-08T09:00:00 America/New_York
	// 2020-01-15T09:00:00 America/New_York
}

// ExampleApplyPatch applies a JSCalendar PatchObject to a document: it replaces
// the title and, via the JSON null sentinel, removes the color.
func ExampleApplyPatch() {
	doc := json.RawMessage(`{"@type":"Event","title":"Weekly sync","color":"red"}`)

	var patch jscalendar.PatchObject
	if err := json.Unmarshal([]byte(`{"title":"Weekly sync (canceled)","color":null}`), &patch); err != nil {
		panic(err)
	}

	out, err := recur.ApplyPatch(doc, patch)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))
	// Output: {"@type":"Event","title":"Weekly sync (canceled)"}
}
