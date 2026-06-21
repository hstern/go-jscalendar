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

```sh
go get github.com/hstern/go-jscalendar
```

```go
import "github.com/hstern/go-jscalendar"
```

## Quickstart

All snippets below are mirrored by runnable `Example` functions
(`example_test.go` and `ical/example_test.go`), so they compile and
their output is verified by `go test`.

### Parse and marshal

`Parse` routes a top-level object to the concrete type named by its
`@type` discriminator — `Event`, `Task`, or `Group`. Marshaling emits
`@type` first and is byte-stable, which keeps round trips deterministic
for interop.

```go
const src = `{
  "@type": "Event",
  "uid": "a8df3f1e-1c2b-4d5e-9f00-112233445566",
  "title": "Team sync",
  "start": "2026-07-01T09:00:00",
  "timeZone": "America/New_York",
  "duration": "PT1H"
}`

obj, err := jscalendar.Parse([]byte(src))
if err != nil {
	log.Fatal(err)
}
ev := obj.(*jscalendar.Event)
fmt.Printf("%s in %s for %s\n", ev.Title, ev.TimeZone, ev.Duration)
// Team sync in America/New_York for PT1H

out, err := json.Marshal(ev) // "@type" first, byte-stable
// {"@type":"Event","uid":"a8df3f1e-...","title":"Team sync",...}
```

### Validate

Decoding is deliberately lenient (Postel's law). The RFC 8984 §4–§5
MUSTs are enforced by an opt-in `Validate` pass, which reports each
violation as a `*ValidationError` carrying the offending JSON path.

```go
ev := &jscalendar.Event{Title: "Missing UID"}

var verr *jscalendar.ValidationError
if errors.As(ev.Validate(), &verr) {
	fmt.Printf("invalid %s\n", verr.Property) // invalid uid
}

ev.UID = "event-1"
fmt.Println(ev.Validate() == nil) // true
```

### Open extensions (the typed-extension pattern)

Unknown members — a vendor extension or a property from a future
revision of the spec, indistinguishable on the wire — round-trip
losslessly through each type's `Extra` map as `json.RawMessage`. Read
one out with `DecodeJSON` and set one with `EncodeJSON`; nothing is
decoded until you ask for it.

```go
// Read an unknown member into a typed value.
type Room struct {
	Building string `json:"building"`
	Floor    int    `json:"floor"`
}
var r Room
err := jscalendar.DecodeJSON(ev.Extra["example.com/room"], &r)
// errors.Is(err, jscalendar.ErrExtensionAbsent) reports a missing member.

// Set an unknown member.
raw, _ := jscalendar.EncodeJSON(map[string]string{"building": "HQ"})
ev.Extra = map[string]json.RawMessage{"example.com/room": raw}
```

### iCalendar conversion

iCalendar interop lives in the `jscalendar/ical` sub-package. Importing
it is what pulls in
[`emersion/go-ical`](https://github.com/emersion/go-ical); the core
`jscalendar` package stays standard-library-only, so consumers who never
need iCalendar take on no extra dependency.

```go
import "github.com/hstern/go-jscalendar/ical"

// iCalendar -> JSCalendar
cal, _ := goical.NewDecoder(r).Decode()
objs, err := ical.FromICal(cal) // []any of *jscalendar.Event / *jscalendar.Task

// JSCalendar -> iCalendar
cal, err = ical.ToICal(ev) // *goical.Calendar; some edges are lossy (see godoc)
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
