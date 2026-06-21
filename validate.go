// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
)

// This file implements the opt-in, strict validation pass for the three
// top-level JSCalendar object types — [Event.Validate], [Task.Validate], and
// [Group.Validate]. Validation is deliberately separate from decoding: the
// UnmarshalJSON methods stay lenient (Postel's law — decode whatever the wire
// gave us), and a consumer who wants the spec's MUST-level guarantees calls
// Validate explicitly (design decision §5). Validate never mutates the object
// and never touches the wire; it only inspects an already-decoded value.
//
// Scope of this pass (RFC 8984, Sections 4–5):
//
//   - "@type" present and naming the correct type for the object.
//   - "uid" present and non-empty (Section 4.1.2).
//   - Event MUST NOT carry a "due" value (Section 5.2.1 makes "due" a Task
//     property; an Event has no Due field, but a "due" member smuggled in
//     through Extra is rejected here).
//   - A floating "start" — a [LocalDateTime] with no sibling "timeZone" — is
//     VALID and is never flagged (Section 1.4.4): the absence of a zone is the
//     spec's floating-time signal, not an error.
//   - Group entries route to [Event]/[Task]; an entry that is neither (an
//     absent/unknown "@type", including a nested "Group") is invalid (Section
//     5.3.1).
//
// Deliberately left to a later validation phase (and structured so it slots in
// without reshaping this file — see validateEvent / validateTask, whose step
// lists are the seam): RFC 6901 pointer well-formedness on recurrenceOverrides
// and localizations keys; the recurrenceId/recurrenceIdTimeZone coupling; the
// custom-time-zone closure (every "/"-prefixed [TimeZoneId] resolving in the
// timeZones map, via [TimeZoneId.ResolvesIn]); and the RecurrenceRule
// "count" XOR "until" exclusivity. Each is an independent check appendable to
// the relevant step list; none changes the [ValidationError] contract below.

// ValidationError reports one violation of an RFC 8984 MUST found by a
// [Event.Validate], [Task.Validate], or [Group.Validate] pass. It names the
// offending property by its JSON path and carries a human-readable message.
//
// Property is the dotted JSON path of the offending member, using the wire
// names (for example "uid", "@type", or "entries[1].@type" for a Group entry),
// so a caller can point at the exact location in the source document. Message
// explains what the MUST requires and how the object violates it.
//
// A single Validate call can surface more than one violation. When it finds
// several, it returns them joined with [errors.Join]: the returned error
// unwraps (via its Unwrap() []error method) to the individual *ValidationError
// values, and [errors.As] against a *ValidationError binds the first one. A
// caller that wants every violation iterates the joined error; a caller that
// only wants to know "is this valid?" checks err != nil. A single violation is
// returned as a bare *ValidationError (not wrapped), so the common one-error
// case needs no unwrapping.
type ValidationError struct {
	// Property is the JSON path of the offending member, in wire names — for
	// example "uid", "@type", or "entries[1].@type".
	Property string
	// Message explains the violated requirement.
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("jscalendar: invalid %s: %s", e.Property, e.Message)
}

// joinValidation collapses a list of collected violations into the single
// error Validate returns: nil for none, the bare *ValidationError for exactly
// one (so the common case needs no unwrapping), and an [errors.Join] of all of
// them otherwise (whose Unwrap() []error yields each *ValidationError, and
// against which errors.As binds the first). Keeping this in one helper lets
// every Validate method share the same return contract.
func joinValidation(errs []*ValidationError) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		joined := make([]error, len(errs))
		for i, e := range errs {
			joined[i] = e
		}
		return errors.Join(joined...)
	}
}

// Validate checks the Event against the RFC 8984 MUSTs this library enforces
// (Sections 4–5) and returns nil when the Event satisfies them. It is an
// opt-in, strict pass: decoding an Event never runs it (UnmarshalJSON stays
// lenient), so a consumer who wants spec-level guarantees calls Validate
// explicitly. Validate does not mutate the Event.
//
// The checks are:
//
//   - "@type" is "Event" (Section 4.1.1). An empty Type is accepted as the
//     implied "Event" (the codec forces it on marshal); any other value is a
//     violation.
//   - "uid" is present and non-empty (Section 4.1.2).
//   - No "due" member is present. "due" is a Task property (Section 5.2.1) and
//     the Event type has no Due field; a "due" carried in [Event.Extra] — for
//     example a Task property mistakenly merged onto an Event — is rejected.
//
// A floating "start" (a [LocalDateTime] with no [Event.TimeZone]) is valid and
// is not flagged: floating time is the spec's intended semantic, not an error
// (Section 1.4.4).
//
// When more than one check fails, the violations are returned joined per the
// [ValidationError] contract; a single failure is returned as a bare
// *ValidationError.
func (e *Event) Validate() error {
	var errs []*ValidationError
	errs = appendIf(errs, validateType(e.Type, typeEvent))
	errs = appendIf(errs, validateUID(e.UID))
	// Event MUST NOT carry "due": it has no Due field, so the only way one
	// reaches an Event is through the open-extension Extra map.
	errs = appendIf(errs, validateNoExtraMember(e.Extra, "due",
		"\"due\" is a Task property and is not permitted on an Event (Section 5.2.1)"))
	// Seam for the later validation phase: recurrence-override pointer
	// well-formedness, recurrenceId/recurrenceIdTimeZone coupling, custom
	// time-zone closure, and RecurrenceRule count/until exclusivity each
	// append their own *ValidationError here without changing the contract.
	return joinValidation(errs)
}

// Validate checks the Task against the RFC 8984 MUSTs this library enforces
// (Sections 4–5) and returns nil when the Task satisfies them. Like
// [Event.Validate] it is opt-in and strict, is never run by decoding, and does
// not mutate the Task.
//
// The checks are:
//
//   - "@type" is "Task" (Section 4.1.1); an empty Type is the implied "Task".
//   - "uid" is present and non-empty (Section 4.1.2).
//
// What this deliberately does NOT enforce, and why: a Task's scheduling shape
// is open by design (Section 5.2). A Task MAY carry a "due" (Section 5.2.1), a
// "start" (Section 5.2.2), and an "estimatedDuration" (Section 5.2.3), any
// combination of them, or none at all — an open-ended to-do with no "due",
// "start", or "estimatedDuration" is a perfectly valid Task (the §6.2 example
// is exactly that). The spec sets no MUST requiring "due", nor one forbidding
// "start" alongside "due", nor a duration/due exclusivity on a Task (the
// exclusivity that bites is the Event-side "due" prohibition, enforced in
// [Event.Validate]). Over-constraining here would reject conformant Tasks, so
// the interplay is left lenient. A floating "start" (no [Task.TimeZone]) is
// valid, as for an Event.
//
// Violations are returned joined per the [ValidationError] contract.
func (t *Task) Validate() error {
	var errs []*ValidationError
	errs = appendIf(errs, validateType(t.Type, typeTask))
	errs = appendIf(errs, validateUID(t.UID))
	// Seam for the later validation phase: the same recurrence/time-zone
	// checks listed in Event.Validate append here for a Task.
	return joinValidation(errs)
}

// Validate checks the Group against the RFC 8984 MUSTs this library enforces
// (Sections 4–5) and returns nil when the Group satisfies them. Like the other
// two it is opt-in and strict, is never run by decoding, and does not mutate
// the Group.
//
// The checks are:
//
//   - "@type" is "Group" (Section 4.1.1); an empty Type is the implied
//     "Group".
//   - "uid" is present and non-empty (Section 4.1.2).
//   - every entry routes to an [Event] or a [Task] (Section 5.3.1): each entry
//     is dispatched on its own "@type" exactly as [Group.Entry] does, and an
//     entry whose "@type" is absent or names anything other than "Event" or
//     "Task" — including a nested "Group", which this library does not admit as
//     an entry — is a violation. A malformed entry (one that is not a JSON
//     object, or that fails to decode) is likewise reported.
//
// Each invalid entry is reported with an "entries[i].@type" property path so a
// caller can find the offending member; multiple bad entries are returned
// joined per the [ValidationError] contract. The entries themselves are not
// recursively validated here — entry-level Validate is the caller's to run on
// the decoded [Event]/[Task] — only their routability is checked, which is the
// Section 5.3.1 MUST.
func (g *Group) Validate() error {
	var errs []*ValidationError
	errs = appendIf(errs, validateType(g.Type, typeGroup))
	errs = appendIf(errs, validateUID(g.UID))
	for i := range g.Entries {
		if v := validateGroupEntry(g, i); v != nil {
			errs = append(errs, v)
		}
	}
	return joinValidation(errs)
}

// appendIf appends v to errs when v is non-nil and returns the (possibly
// unchanged) slice, so each Validate method reads as a flat sequence of checks
// without a conditional per step.
func appendIf(errs []*ValidationError, v *ValidationError) []*ValidationError {
	if v != nil {
		errs = append(errs, v)
	}
	return errs
}

// validateType checks that a top-level object's "@type" discriminator matches
// the type being validated (RFC 8984, Section 4.1.1). An empty got is accepted
// as the implied want: the codec forces the discriminator onto the wire on
// marshal, so a zero Type is the not-yet-stamped form of the correct type, not
// a wrong one. Any other mismatch is a violation naming "@type".
func validateType(got, want string) *ValidationError {
	if got == "" || got == want {
		return nil
	}
	return &ValidationError{
		Property: "@type",
		Message:  fmt.Sprintf("must be %q, got %q (Section 4.1.1)", want, got),
	}
}

// validateUID checks that "uid" is present and non-empty (RFC 8984, Section
// 4.1.2), the universal required member of every top-level object. The zero
// value of the Go field is the empty string, which is exactly the "absent or
// empty" case the spec forbids, so this one check covers both.
func validateUID(uid string) *ValidationError {
	if uid != "" {
		return nil
	}
	return &ValidationError{
		Property: "uid",
		Message:  "is required and must be non-empty (Section 4.1.2)",
	}
}

// validateNoExtraMember reports a violation when name is present as a key in an
// object's Extra map — the open-extension catch-all the codec fills with
// members that have no typed field. It is how a property forbidden on a type
// (such as "due" on an Event) is detected: the type has no field for it, so the
// only place it can land is Extra. message is the reason embedded in the
// returned [ValidationError], and name is also its Property path.
func validateNoExtraMember(extra map[string]json.RawMessage, name, message string) *ValidationError {
	if _, present := extra[name]; !present {
		return nil
	}
	return &ValidationError{Property: name, Message: message}
}

// validateGroupEntry checks that the Group entry at index i routes to an
// [Event] or a [Task] (RFC 8984, Section 5.3.1). It reuses the package's
// "@type" peek so its notion of routability is identical to [Group.Entry]'s:
// an entry that is not a JSON object, whose "@type" is absent, or whose "@type"
// is neither "Event" nor "Task" (a "Group" entry included) is reported with an
// "entries[i].@type" property path. It returns nil for a routable entry.
func validateGroupEntry(g *Group, i int) *ValidationError {
	prop := fmt.Sprintf("entries[%d].@type", i)
	typ, _, err := peekType(g.Entries[i])
	if err != nil {
		return &ValidationError{
			Property: prop,
			Message:  "entry is not a valid JSON object (Section 5.3.1)",
		}
	}
	switch typ {
	case typeEvent, typeTask:
		return nil
	case "":
		return &ValidationError{
			Property: prop,
			Message:  "entry has no \"@type\"; must be \"Event\" or \"Task\" (Section 5.3.1)",
		}
	default:
		return &ValidationError{
			Property: prop,
			Message: fmt.Sprintf(
				"entry @type %q must be \"Event\" or \"Task\" (Section 5.3.1)", typ),
		}
	}
}
