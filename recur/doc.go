// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

// Package recur expands a recurring JSCalendar [jscalendar.Event] or
// [jscalendar.Task] into the concrete occurrences that fall within a window
// (RFC 8984, Section 4.3). It evaluates "recurrenceRules", subtracts
// "excludedRecurrenceRules", and applies "recurrenceOverrides" — including the
// PatchObject application layer (Section 3.3) that the core jscalendar package
// deliberately leaves to a higher level. Each occurrence is returned as an
// independent object with its "recurrenceId" set and its override patch applied.
//
// The entry points are [Occurrences] for an Event and [TaskOccurrences] for a
// Task; [ApplyPatch] exposes the PatchObject apply layer on its own.
//
// # Window semantics
//
// The window is half-open: [Occurrences] returns the instances whose
// recurrence id lies in [from, until) — at or after from, strictly before
// until. The bound is tested against the recurrence id (the occurrence's
// nominal start as the rules generate it), not against a start an override may
// have moved. A floating series (one with no time zone) is placed on the
// absolute timeline in UTC for the sole purpose of applying the window; the
// occurrences themselves remain floating wall-clock values.
//
// Daylight-saving transitions are handled by keeping the wall-clock time
// constant across the boundary, the way iCalendar and JSCalendar intend: a
// daily 09:00 event in a zone that springs forward still fires at 09:00 local
// time, with its UTC offset — and therefore its absolute instant — shifting by
// the transition amount.
//
// # Dependency choice
//
// Recurrence expansion is delegated to github.com/teambition/rrule-go, an
// RFC 5545 RRULE engine. The choice is deliberate and is confined to this
// package:
//
//   - rrule-go is already in the module graph as an indirect dependency of the
//     iCalendar converter's go-ical dependency, so leaning on it here adds no
//     new module to a consumer's go.sum — it only promotes an existing indirect
//     dependency to a direct one within this sub-package.
//   - The expansion math the spec inherits from RRULE — positional "byDay", the
//     "bySetPosition" interaction with the other "by*" filters, "byWeekNo" — is
//     subtle enough that a battle-tested engine is materially less risky than a
//     hand-rolled one, in line with the "correct for the common case beats
//     incomplete and fragile" posture.
//   - The dependency does NOT enter the core jscalendar package's import graph.
//     The core models recurrence rules but never expands them, so a consumer
//     who imports only github.com/hstern/go-jscalendar pulls in no recurrence
//     engine. Verify with: go list -deps github.com/hstern/go-jscalendar | grep
//     rrule (which prints nothing). The coupling lives entirely behind this
//     package's API, which speaks in jscalendar and time types, never rrule
//     ones.
//
// # Limitations
//
// This is a first cut aimed at the common Gregorian case. The following are
// explicit, documented gaps rather than silent approximations:
//
//   - RSCALE / non-Gregorian calendars (RFC 7529): a rule whose "rscale" names
//     anything but the Gregorian calendar, and the leap-month "byMonth" form
//     (a trailing "L"), return an error rather than a wrong result. The "skip"
//     property is meaningful only with a non-Gregorian rscale and is otherwise
//     ignored.
//   - Custom, "/"-prefixed time zones resolve only when their embedded
//     definition reduces to a single UTC offset; a custom zone that encodes its
//     own daylight-saving transitions returns an error. IANA-named zones get
//     full transition handling through the Go time package.
//   - A Task is anchored on its "start"; a task without a "start" cannot be
//     expanded.
//
// [jscalendar.Event]: https://pkg.go.dev/github.com/hstern/go-jscalendar#Event
// [jscalendar.Task]: https://pkg.go.dev/github.com/hstern/go-jscalendar#Task
package recur
