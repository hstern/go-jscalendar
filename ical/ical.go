// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"errors"

	goical "github.com/emersion/go-ical"
)

// ErrNotImplemented is returned by the conversion functions whose bodies have
// not yet been written. It is a sentinel: callers may test for it with
// [errors.Is] to distinguish "this conversion direction is not built yet" from
// a genuine conversion failure once the implementations land.
var ErrNotImplemented = errors.New("jscalendar/ical: not implemented")

// FromICal converts an iCalendar calendar into its JSCalendar objects.
//
// Each top-level component of cal (a VEVENT, VTODO, or the VCALENDAR taken as a
// group) maps to one JSCalendar object, returned in document order. Every
// element of the returned slice holds one of the concrete pointer types the
// parent package's parse routing produces — a *jscalendar.Event,
// *jscalendar.Task, or *jscalendar.Group — matching the contract of
// jscalendar.Parse. Callers type-switch on each element to recover the concrete
// type.
//
// The mapping follows the calext jscalendar-icalendar property correspondence:
// for example DTSTART with a TZID parameter splits into the JSCalendar start
// and timeZone properties, RRULE becomes a recurrence rule, VALARM becomes an
// alert, and VTIMEZONE definitions populate the object's timeZones map.
//
// FromICal is a skeleton and currently returns [ErrNotImplemented]; the
// conversion is implemented by a subsequent phase-6 change.
//
//nolint:revive // cal is named for the published godoc signature; the skeleton body ignores it until a subsequent phase-6 change.
func FromICal(cal *goical.Calendar) ([]any, error) {
	return nil, ErrNotImplemented
}

// ToICal converts one or more JSCalendar objects into a single iCalendar
// calendar.
//
// Each argument must be one of the concrete pointer types the parent package
// produces — a *jscalendar.Event, *jscalendar.Task, or *jscalendar.Group —
// typically obtained from jscalendar.Parse. Each object becomes one top-level
// iCalendar component within the returned [goical.Calendar]; a Group's entries
// expand into their respective components. An object of an unsupported dynamic
// type is a conversion error once the implementation lands.
//
// The mapping is the inverse of [FromICal] and follows the same calext
// jscalendar-icalendar correspondence. Some JSCalendar constructs have no exact
// iCalendar counterpart; the implementing change documents those lossy edges.
//
// ToICal is a skeleton and currently returns [ErrNotImplemented]; the
// conversion is implemented by a subsequent phase-6 change.
//
//nolint:revive // objs is named for the published godoc signature; the skeleton body ignores it until a subsequent phase-6 change.
func ToICal(objs ...any) (*goical.Calendar, error) {
	return nil, ErrNotImplemented
}
