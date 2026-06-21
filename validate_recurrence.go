// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"fmt"
	"slices"
)

// This file holds the phase-two validation checks that the seam in
// [Event.Validate] and [Task.Validate] (validate.go) reserves space for:
// recurrence-override key and pointer well-formedness, the localizations
// pointer well-formedness, the custom-time-zone closure, and the
// RecurrenceRule cross-field constraints. Each is exposed as a free function
// returning a []*ValidationError, so the Event and Task validators — whose
// recurrence/time-zone fields are identical — append the same checks without
// either type reaching into the other.
//
// The checks honor the same RFC 8984 clauses tracked here:
//
//   - recurrenceOverrides keys are LocalDateTimes (Section 4.3.5): each map key
//     is the overridden occurrence's "start" in the master's zone, so a key
//     that does not parse as a [LocalDateTime] is a violation.
//   - recurrenceOverrides and localizations values are PatchObjects (Sections
//     4.3.5, 4.6.1): every pointer key must be well-formed per RFC 6901 and
//     must not target the root "@type" or "uid", reusing [ValidatePatchKey].
//   - the custom-time-zone closure (Section 4.7.1): every "/"-prefixed
//     [TimeZoneId] referenced by the object resolves to a key in its timeZones
//     map.
//   - RecurrenceRule constraints (Section 4.3.1): "frequency" is required and
//     must be one of the defined values, and "count" and "until" are mutually
//     exclusive.

// validateRecurrenceOverrides checks the recurrenceOverrides map of an Event or
// Task (RFC 8984, Section 4.3.5). Each key MUST be a valid [LocalDateTime] — the
// overridden occurrence's start in the master's time zone — and each value is a
// [PatchObject] whose pointer keys MUST be well-formed RFC 6901 pointers that do
// not target the root "@type" or "uid". Violations are returned in a stable
// order: keys are visited sorted, and within a key the override-key check
// precedes the pointer checks.
func validateRecurrenceOverrides(overrides map[string]PatchObject) []*ValidationError {
	var errs []*ValidationError
	for _, key := range sortedKeys(overrides) {
		prop := fmt.Sprintf("recurrenceOverrides/%s", key)
		if _, err := ParseLocalDateTime(key); err != nil {
			errs = append(errs, &ValidationError{
				Property: prop,
				Message: fmt.Sprintf(
					"key must be a valid LocalDateTime occurrence start (Section 4.3.5): %v", err),
			})
		}
		errs = append(errs, validatePatchPointers(prop, overrides[key])...)
	}
	return errs
}

// validateLocalizations checks the localizations map of an Event or Task (RFC
// 8984, Section 4.6.1). The keys are language tags and are not constrained here;
// each value is a [PatchObject] whose pointer keys MUST be well-formed RFC 6901
// pointers that do not target the root "@type" or "uid". Keys are visited in
// sorted order for a deterministic violation sequence.
func validateLocalizations(localizations map[string]PatchObject) []*ValidationError {
	var errs []*ValidationError
	for _, lang := range sortedKeys(localizations) {
		prop := fmt.Sprintf("localizations/%s", lang)
		errs = append(errs, validatePatchPointers(prop, localizations[lang])...)
	}
	return errs
}

// validatePatchPointers checks every pointer key in a [PatchObject] for RFC
// 6901 well-formedness and the JSCalendar key rules, reusing [ValidatePatchKey]
// (defined in patch.go for the standalone PatchObject.Validate pass) so the
// notion of a valid patch pointer is identical here and there. The patch's own
// rule already forbids the root "@type"; the additional root "uid" prohibition
// the override/localization context imposes is enforced on top.
//
// parent is the JSON path of the enclosing PatchObject (for example
// "recurrenceOverrides/2020-01-08T14:00:00"); each violation's Property is that
// path joined to the offending pointer so a caller can locate it exactly. A
// [*PatchError] from ValidatePatchKey is rewrapped as a [*ValidationError] to
// keep Validate's single error contract.
func validatePatchPointers(parent string, patch PatchObject) []*ValidationError {
	var errs []*ValidationError
	for _, key := range sortedKeys(patch) {
		prop := fmt.Sprintf("%s/%s", parent, key)
		if key == "uid" {
			errs = append(errs, &ValidationError{
				Property: prop,
				Message:  "patch pointer must not target the root \"uid\" (Section 4.3.5)",
			})
			continue
		}
		if err := ValidatePatchKey(key); err != nil {
			errs = append(errs, &ValidationError{
				Property: prop,
				Message:  fmt.Sprintf("malformed patch pointer (RFC 6901): %v", err),
			})
		}
	}
	return errs
}

// validateTimeZoneClosure checks the custom-time-zone closure (RFC 8984,
// Section 4.7.1): every "/"-prefixed [TimeZoneId] the object references MUST
// resolve to a key in its timeZones map. IANA names (not "/"-prefixed) are not
// checked against the time-zone database — only the closure rule applies.
//
// The references checked are the object's top-level timeZone and the timeZone
// of each [Location] in its locations map. A reference that is custom but absent
// from timeZones is a violation; references are visited so that the top-level
// timeZone is reported before location references, and locations in sorted Id
// order, for a deterministic sequence.
func validateTimeZoneClosure(
	topLevel TimeZoneId,
	locations map[Id]Location,
	timeZones map[TimeZoneId]TimeZone,
) []*ValidationError {
	present := func(k TimeZoneId) bool {
		_, found := timeZones[k]
		return found
	}

	var errs []*ValidationError
	if v := checkTimeZoneRef("timeZone", topLevel, present); v != nil {
		errs = append(errs, v)
	}
	for _, id := range sortedKeys(locations) {
		prop := fmt.Sprintf("locations/%s/timeZone", id)
		if v := checkTimeZoneRef(prop, locations[id].TimeZone, present); v != nil {
			errs = append(errs, v)
		}
	}
	return errs
}

// checkTimeZoneRef reports a closure violation for a single TimeZoneId
// reference at the given JSON path, or nil when the reference is empty, an IANA
// name, or a custom id that resolves via present. present is the timeZones-map
// membership test supplied by [validateTimeZoneClosure].
func checkTimeZoneRef(prop string, ref TimeZoneId, present func(TimeZoneId) bool) *ValidationError {
	if ref == "" || ref.ResolvesIn(present) {
		return nil
	}
	return &ValidationError{
		Property: prop,
		Message: fmt.Sprintf(
			"custom time zone %q is not defined in timeZones (Section 4.7.1)", string(ref)),
	}
}

// validateRecurrenceRules checks every rule in a recurrenceRules or
// excludedRecurrenceRules list against the RFC 8984 Section 4.3.1 constraints:
// "frequency" is required and must name a defined [Frequency], and "count" and
// "until" are mutually exclusive. field is the wire name of the list
// ("recurrenceRules" or "excludedRecurrenceRules") so each violation's Property
// pinpoints the offending rule by index.
func validateRecurrenceRules(field string, rules []RecurrenceRule) []*ValidationError {
	var errs []*ValidationError
	for i, rule := range rules {
		errs = append(errs, validateRecurrenceRule(fmt.Sprintf("%s[%d]", field, i), rule)...)
	}
	return errs
}

// validateRecurrenceRule checks a single [RecurrenceRule] against the Section
// 4.3.1 constraints. prop is the rule's JSON path. The frequency check precedes
// the count/until exclusivity check so a rule that is both unfrequent and
// over-bounded reports the missing frequency first.
func validateRecurrenceRule(prop string, rule RecurrenceRule) []*ValidationError {
	var errs []*ValidationError
	if rule.Frequency == "" {
		errs = append(errs, &ValidationError{
			Property: prop + ".frequency",
			Message:  "is required (Section 4.3.1)",
		})
	} else if !rule.Frequency.IsValid() {
		errs = append(errs, &ValidationError{
			Property: prop + ".frequency",
			Message: fmt.Sprintf(
				"%q is not a valid frequency (Section 4.3.1)", string(rule.Frequency)),
		})
	}
	if rule.HasCount() && rule.HasUntil() {
		errs = append(errs, &ValidationError{
			Property: prop,
			Message:  "\"count\" and \"until\" are mutually exclusive (Section 4.3.1)",
		})
	}
	return errs
}

// validateEmbeddedTimeZoneRules checks the recurrence rules embedded in the
// object's custom time-zone definitions (RFC 8984, Section 4.7.3): each
// [TimeZoneRule] in a [TimeZone]'s standard and daylight transition lists
// carries its own [RecurrenceRule]s, subject to the same Section 4.3.1
// constraints as the object's own rules. The timeZones map is visited in sorted
// [TimeZoneId] order, standard before daylight, for a deterministic violation
// sequence.
func validateEmbeddedTimeZoneRules(timeZones map[TimeZoneId]TimeZone) []*ValidationError {
	var errs []*ValidationError
	for _, id := range sortedKeys(timeZones) {
		tz := timeZones[id]
		base := fmt.Sprintf("timeZones/%s", id)
		errs = append(errs, validateTransitionRules(base+"/standard", tz.Standard)...)
		errs = append(errs, validateTransitionRules(base+"/daylight", tz.Daylight)...)
	}
	return errs
}

// validateTransitionRules checks the RecurrenceRules of each transition in a
// [TimeZone]'s standard or daylight list. prefix is the JSON path of the
// transition list ("timeZones/<id>/standard" or ".../daylight"); each violation
// is pinpointed to the transition and rule by index.
func validateTransitionRules(prefix string, transitions []TimeZoneRule) []*ValidationError {
	var errs []*ValidationError
	for i, transition := range transitions {
		for j, rule := range transition.RecurrenceRules {
			errs = append(errs,
				validateRecurrenceRule(fmt.Sprintf("%s[%d].recurrenceRules[%d]", prefix, i, j), rule)...)
		}
	}
	return errs
}

// recurrenceScope bundles the recurrence and time-zone fields an Event and a
// Task carry identically, so the phase-two checks run against one value rather
// than a long parameter list. [Event.Validate] and [Task.Validate] each build a
// recurrenceScope from their own fields and hand it to
// [validateRecurrenceAndTimeZones]; the shared shape is what lets the two types
// reuse the same checks without either reaching into the other.
type recurrenceScope struct {
	timeZone                TimeZoneId
	locations               map[Id]Location
	timeZones               map[TimeZoneId]TimeZone
	recurrenceRules         []RecurrenceRule
	excludedRecurrenceRules []RecurrenceRule
	recurrenceOverrides     map[string]PatchObject
	localizations           map[string]PatchObject
}

// validateRecurrenceAndTimeZones runs the full phase-two check set for an Event
// or Task — recurrence-override keys and pointers, localization pointers, the
// custom-time-zone closure, the recurrence-rule constraints, and the embedded
// time-zone rules — over the shared [recurrenceScope]. Collecting them in one
// helper keeps [Event.Validate] and [Task.Validate] reduced to a single seam
// call each, and fixes the order in which the groups are reported.
func validateRecurrenceAndTimeZones(s recurrenceScope) []*ValidationError {
	var errs []*ValidationError
	errs = append(errs, validateRecurrenceRules("recurrenceRules", s.recurrenceRules)...)
	errs = append(errs,
		validateRecurrenceRules("excludedRecurrenceRules", s.excludedRecurrenceRules)...)
	errs = append(errs, validateRecurrenceOverrides(s.recurrenceOverrides)...)
	errs = append(errs, validateLocalizations(s.localizations)...)
	errs = append(errs, validateTimeZoneClosure(s.timeZone, s.locations, s.timeZones)...)
	errs = append(errs, validateEmbeddedTimeZoneRules(s.timeZones)...)
	return errs
}

// sortedKeys returns the keys of m sorted ascending. The validation helpers use
// it to visit maps in a stable order so the sequence of reported violations is
// deterministic regardless of Go's randomized map iteration. The key constraint
// is ~string so it serves the recurrenceOverrides/localizations maps (keyed by
// string) as well as the locations and timeZones maps (keyed by the string-based
// [Id] and [TimeZoneId]).
func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
