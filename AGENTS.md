# Contributing to go-jscalendar

This file orients contributors (human and agent) to the conventions this
repository expects. It describes the library on its own terms.

## What this is

A Go library implementing [RFC 8984 — JSCalendar: A JSON Representation
of Calendar Data](https://www.rfc-editor.org/rfc/rfc8984.html). The core
package `jscalendar` is the typed object model and codec; the
`jscalendar/ical` subpackage converts to and from iCalendar.

## Go version

Go 1.26 or newer (see `go.mod`).

## Conventions

- **Wire fidelity over ergonomic shortcuts.** If the spec says X, the
  library round-trips X verbatim. The value-type grammars (`Duration`,
  `LocalDateTime` vs `UTCDateTime`, `PatchObject`, `RecurrenceRule`) are
  spec grammars, not Go conveniences — `Duration` is not
  `time.Duration`, and `LocalDateTime` carries no zone.
- **Lenient unmarshal, strict marshal.** Decode whatever the wire gives
  you (Postel's law); fail fast only at the marshal boundary on missing
  required fields. Stricter checking is opt-in via `Validate`.
- **Byte-stable round-trip.** Unknown members are preserved and
  re-emitted; `@type` is always emitted first. Open-extension fields are
  `json.RawMessage`, not `map[string]any`.
- **Every exported symbol carries godoc** citing the spec section it
  implements.

## Copyright header

Every `.go` file (including tests) begins with:

```go
// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0
```

Contributors are recorded in the git history, not in per-file bylines.

## Commit messages

Imperative present-tense title (≤72 chars, no trailing period, no
`feat:`/`fix:` prefix), followed by a body paragraph explaining *why*
the change exists — the spec clause honored, the bug observed, the
behavior changed. Use public references only (RFC section numbers, RFC
numbers, commit SHAs). Trivial changes (typos, dependency patch bumps,
the bootstrap scaffold) may be a single line.

## Dependencies

- **Runtime: standard library only, with one exception class.**
  A non-stdlib runtime dependency is acceptable only when (a) it
  implements a validator no reasonable hand-coding could match
  (libphonenumber-class data: country code numbering plan,
  per-country length rules, IDN normalization tables); (b) it is
  well-maintained and widely used in the Go ecosystem; and
  (c) the alternative is the library quietly accepting input the
  spec rejects. Any other runtime dep needs a discussion and a
  justification in the PR description. Default answer is still
  "no" — the bar is "the spec demands data we cannot reasonably
  ship ourselves."
  - **The one standing exception** is the `jscalendar/ical` subpackage,
    which depends on [`github.com/emersion/go-ical`](https://github.com/emersion/go-ical)
    rather than re-implementing RFC 5545 parsing. This dependency is
    confined to that subpackage: importers of the core `jscalendar`
    package never pull it into their `go.sum`.
- **Tests: standard library only by default.** Test-only deps
  still need a one-line justification.
- **Build-time tooling: unconstrained.** Generators, linters,
  release tooling, and similar live under `tools/` (separate
  `go.mod`) or are invoked via `go run` with a pinned version;
  they never end up in library users' `go.sum`.
- **`go.mod`**: keep the `module` path stable at
  `github.com/hstern/go-jscalendar` (no `/vN` suffix for v0.x/v1.x — Go
  SemVer rule). Major-version bumps follow the `go-jose` branch
  pattern.

## CI

`ci.yml` builds, then fans out to `static` (build + `go vet`),
`test` (`go test -race`), `lint` (`golangci-lint`), and `interop` (the
iCalendar-conversion conformance corpus). `vuln.yml` runs `govulncheck`
on `main` and on a daily schedule. All must pass before merge.
