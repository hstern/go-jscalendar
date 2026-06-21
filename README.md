# go-jscalendar

[![CI](https://github.com/hstern/go-jscalendar/actions/workflows/ci.yml/badge.svg)](https://github.com/hstern/go-jscalendar/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/hstern/go-jscalendar.svg)](https://pkg.go.dev/github.com/hstern/go-jscalendar)

A Go library implementing **[RFC 8984 — JSCalendar: A JSON Representation
of Calendar Data](https://www.rfc-editor.org/rfc/rfc8984.html)**.

JSCalendar is the JSON-native representation of calendar data — the
modern counterpart to iCalendar (RFC 5545). A top-level object is an
`Event`, a `Task`, or a `Group`, discriminated by its `@type` member,
with typed properties for scheduling, recurrence, time zones,
participants, alerts, and localizations. It is richer and less ambiguous
than iCalendar exactly where iCalendar is painful: structured recurrence
rules instead of an opaque `RRULE` string, IANA time zone names plus
embedded custom zones, and first-class localizations.

```go
import "github.com/hstern/go-jscalendar"
```

## Status

**Pre-publication — under construction toward `v0.1.0`.** The exported
API is not yet stable. The core object model is standard-library-only;
the iCalendar conversion lives in the `jscalendar/ical` subpackage and
is the only part of the module that depends on
[`emersion/go-ical`](https://github.com/emersion/go-ical).

## Scope

In scope for `v0.1.0`:

- The typed object model (RFC 8984 §4–§5): `Event`, `Task`, `Group` and
  their common, scheduling, and recurrence properties.
- The property value types (§1.4): `LocalDateTime`, `UTCDateTime`,
  `Duration`, `SignedDuration`, `TimeZoneId`, `Id`, `PatchObject`,
  `RecurrenceRule`.
- A byte-stable JSON codec that round-trips unknown members losslessly
  and emits `@type` first for interop stability.
- `Validate` for the §4–§5 MUST requirements.
- iCalendar ⇄ JSCalendar conversion in `jscalendar/ical`.

Out of scope: JMAP transport, and expansion of recurrence rules into
concrete occurrences (the library models the rule; expanding a series
over a window is the caller's job).

## License

[Apache-2.0](LICENSE).
