// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

// Package jscalendar implements RFC 8984, JSCalendar: A JSON
// Representation of Calendar Data.
//
// JSCalendar is the JSON-native representation of calendar data — the
// modern counterpart to iCalendar (RFC 5545). A top-level object is an
// [Event], a [Task], or a [Group], discriminated by its "@type" member,
// with typed properties for scheduling, recurrence, time zones,
// participants, alerts, and localizations.
//
// This package (the core object model) depends only on the standard
// library. The iCalendar conversion lives in the separate subpackage
// jscalendar/ical, which is the only part of the module that depends on
// github.com/emersion/go-ical; importing the core model stays
// dependency-free.
//
// The library is under construction; see the build plan for the phased
// path to v0.1.0. The exported surface is not yet stable.
package jscalendar

// SpecVersion is the version of RFC 8984 this build implements.
//
// JSCalendar tracks the published RFC rather than a draft revision, so
// the value has no minor or patch component.
const SpecVersion = "RFC 8984"
