# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `recur` sub-package: expand `recurrenceRules`, `excludedRecurrenceRules`,
  and `recurrenceOverrides` into concrete occurrences over a half-open window,
  including a `PatchObject` application layer (`ApplyPatch`). The recurrence
  engine dependency is confined to this package and never enters the core
  package's import graph.

## [0.1.0]

### Added

- Repository bootstrap: module definition, license, CI, and the stub
  package exposing `SpecVersion`.

[Unreleased]: https://github.com/hstern/go-jscalendar/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hstern/go-jscalendar/releases/tag/v0.1.0
