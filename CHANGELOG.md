# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0]

### Added

- `recur` sub-package: expand `recurrenceRules`, `excludedRecurrenceRules`,
  and `recurrenceOverrides` into concrete occurrences over a half-open window,
  including a `PatchObject` application layer (`ApplyPatch`). The recurrence
  engine dependency is confined to this package and never enters the core
  package's import graph.
- `jmap` sub-package: a `CalendarEvent` object adapter mapping the JMAP
  Calendars `CalendarEvent` to and from `jscalendar.Event`, with a byte-stable
  codec and the JMAP-specific members (`calendarIds`, `isDraft`, `utcStart`,
  and the rest) layered onto the JSCalendar Event. Object mapping only —
  the JMAP transport methods are out of scope. Standard-library only.

## [0.1.0]

### Added

- The RFC 8984 (JSCalendar) typed object model: `Event`, `Task`, and `Group`
  with the common, scheduling, and recurrence properties plus the sub-objects
  (participants, locations, virtual locations, alerts, links, related-to,
  localizations, and embedded time zones).
- The §1.4 property value types: `LocalDateTime`, `UTCDateTime`, `Duration`
  and `SignedDuration` (the JSCalendar grammar, not `time.Duration`), `Id`,
  `TimeZoneId`, `PatchObject` (JSON-Pointer-keyed), and `RecurrenceRule`.
- A byte-stable JSON codec that emits `@type` first and round-trips unknown
  members losslessly through an open-extension `Extra` field, with `Parse`
  dispatching on the `@type` discriminator.
- `Validate` for the §4–§5 MUSTs (required properties, recurrence-override
  keys, JSON-Pointer validity, custom time-zone closure, and recurrence-rule
  constraints).
- `ical` sub-package: iCalendar ⇄ JSCalendar conversion (`FromICal` /
  `ToICal`), depending on `emersion/go-ical` and confined to that subpackage
  so the core object model stays standard-library-only.
- Repository bootstrap: module definition, Apache-2.0 license, and CI.

[Unreleased]: https://github.com/hstern/go-jscalendar/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hstern/go-jscalendar/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/hstern/go-jscalendar/releases/tag/v0.1.0
