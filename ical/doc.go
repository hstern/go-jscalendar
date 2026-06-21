// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

// Package ical converts calendar data between iCalendar (RFC 5545) and
// JSCalendar (RFC 8984).
//
// iCalendar is parsed and serialized through github.com/emersion/go-ical;
// the JSCalendar side is the parent module's jscalendar package
// (github.com/hstern/go-jscalendar). The conversion follows the IETF calext
// working group's "JSCalendar: Converting from and to iCalendar"
// (draft-ietf-calext-jscalendar-icalendar) property mapping, which defines how
// each iCalendar component and property corresponds to its JSCalendar
// counterpart.
//
// # Dependency isolation
//
// This package is the only package in the module that imports
// github.com/emersion/go-ical. The core jscalendar package depends on the
// standard library alone; importing it does not pull go-ical into a consumer's
// module graph. Only code that imports github.com/hstern/go-jscalendar/ical
// takes on the go-ical dependency. This boundary is deliberate: it lets users
// of the typed JSCalendar object model stay dependency-free while still
// offering iCalendar interoperability to those who opt in.
//
// To keep the package name (ical) from colliding with the go-ical import
// (whose package is also named ical), this package aliases the import as
// goical.
//
// # Status
//
// The conversion functions are skeletons: [FromICal] and [ToICal] currently
// return [ErrNotImplemented]. Their behavior is filled in by subsequent
// phase-6 changes — FromICal (iCalendar to JSCalendar) and ToICal (JSCalendar
// to iCalendar) land separately. The signatures are published now so the
// package compiles, participates in CI, and pins the API the implementations
// will satisfy.
package ical
