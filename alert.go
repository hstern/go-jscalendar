// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// This file defines the Alert sub-object (RFC 8984, Section 4.5.2), the value
// type of the "alerts" property on [Event] and [Task], together with its
// polymorphic "trigger" union (Section 4.5.1): a trigger is either an
// OffsetTrigger, firing a [SignedDuration] before or after the object's start
// or end, or an AbsoluteTrigger, firing at a fixed [UTCDateTime].
//
// Like the other sub-objects, Alert emits its "@type" discriminator first and
// round-trips unknown members losslessly through an Extra field, reusing the
// shared marshalKnown / unmarshalKnown seam (codec.go, extra.go).
//
// The trigger union is the load-bearing part. Routing is on the trigger's
// own inner "@type" member on the wire — never on a Go type assertion — so a
// trigger whose "@type" is neither "OffsetTrigger" nor "AbsoluteTrigger"
// (a vendor or future-spec trigger kind) is still preserved verbatim rather
// than dropped or rejected. [Trigger] is a small wrapper that holds the
// decoded concrete value when the kind is recognized and the original raw
// bytes otherwise, so every trigger round-trips byte-stably.

// Alert is a JSCalendar "Alert" (RFC 8984, Section 4.5.2): a reminder
// associated with an [Event] or [Task], carried as a value of an alerts map
// keyed by a stable [Id].
//
// Every member is optional and omitted from the marshaled object when zero.
// Decoding never rejects an Alert; such checks belong to the opt-in
// validation pass.
type Alert struct {
	// Type is the "@type" discriminator and MUST equal "Alert" (Section
	// 4.5.2). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Trigger defines when the alert fires (Section 4.5.2): an OffsetTrigger
	// relative to the object's start or end, or an AbsoluteTrigger at a fixed
	// instant. The union is discriminated by the trigger's own "@type"; an
	// unrecognized trigger kind round-trips verbatim. The zero Trigger is
	// omitted from the marshaled object via omitzero, which honors the
	// [Trigger.IsZero] method (encoding/json's omitempty does not treat a
	// struct value as empty).
	Trigger Trigger `json:"trigger,omitzero"`
	// Acknowledged is the [UTCDateTime] at which the user most recently
	// acknowledged (dismissed) the alert (Section 4.5.2); unset when the
	// alert has not been acknowledged.
	Acknowledged *UTCDateTime `json:"acknowledged,omitempty"`
	// RelatedTo relates this alert to other alerts, keyed by the related
	// alert's identifier (Section 4.5.2). Each value is a [Relation].
	RelatedTo map[string]Relation `json:"relatedTo,omitempty"`
	// Action is how the alert should be presented when it fires: "display"
	// to show it to the user, or "email" to send a message (Section 4.5.2);
	// default "display". The value is open: an unrecognized action
	// round-trips.
	Action string `json:"action,omitempty"`

	// Extra holds Alert members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// alertType is the "@type" discriminator value for an Alert (RFC 8984,
// Section 4.5.2), forced onto the wire and the decoded value by the codec.
const alertType = "Alert"

// alertAlias carries the field set and struct tags of [Alert] but none of its
// methods, so encoding/json uses its default struct codec rather than
// recursing into MarshalJSON / UnmarshalJSON.
type alertAlias Alert

// MarshalJSON encodes the Alert with "@type":"Alert" as the first member, the
// known properties in declaration order, and any [Alert.Extra] members in
// sorted key order, byte-stable throughout. A zero or mismatched [Alert.Type]
// is normalized to "Alert".
func (a Alert) MarshalJSON() ([]byte, error) {
	alias := alertAlias(a)
	alias.Type = alertType
	return marshalKnown(alias, alertType, a.Extra)
}

// UnmarshalJSON decodes a JSON object into the Alert, tolerant of member order
// and a missing "@type". [Alert.Type] is set to "Alert" after a successful
// decode regardless of the wire value. Members with no corresponding known
// property are captured into [Alert.Extra].
func (a *Alert) UnmarshalJSON(data []byte) error {
	var alias alertAlias
	extra, err := unmarshalKnown(data, &alias, alertType, func() { alias.Type = alertType })
	if err != nil {
		return err
	}
	*a = Alert(alias)
	a.Extra = extra
	return nil
}

// The "@type" discriminator values for the two recognized alert trigger
// kinds (RFC 8984, Section 4.5.1).
const (
	offsetTriggerType   = "OffsetTrigger"
	absoluteTriggerType = "AbsoluteTrigger"
)

// OffsetTrigger is a JSCalendar "OffsetTrigger" (RFC 8984, Section 4.5.1): an
// alert trigger that fires a [SignedDuration] before (negative) or after
// (positive) the anchor selected by RelativeTo.
type OffsetTrigger struct {
	// Offset is the signed duration from the anchor at which the alert fires
	// (Section 4.5.1); a negative offset fires before the anchor.
	Offset SignedDuration
	// RelativeTo selects the anchor the Offset is measured from: "start" or
	// "end" (Section 4.5.1); default "start". The value is open.
	RelativeTo string
}

// offsetTriggerWire is the JSON shape of an [OffsetTrigger]: the "@type"
// discriminator first, then the known members. It is a distinct type from
// OffsetTrigger so the value type stays free of struct tags and so the
// discriminator can be forced on marshal.
type offsetTriggerWire struct {
	Type       string         `json:"@type"`
	Offset     SignedDuration `json:"offset"`
	RelativeTo string         `json:"relativeTo,omitempty"`
}

// triggerKind reports the trigger's "@type" discriminator value, here always
// "OffsetTrigger".
func (OffsetTrigger) triggerKind() string { return offsetTriggerType }

// marshalTrigger encodes the OffsetTrigger with "@type" first, forcing the
// discriminator regardless of how the value was built.
func (o OffsetTrigger) marshalTrigger() ([]byte, error) {
	return json.Marshal(offsetTriggerWire{
		Type:       offsetTriggerType,
		Offset:     o.Offset,
		RelativeTo: o.RelativeTo,
	})
}

// AbsoluteTrigger is a JSCalendar "AbsoluteTrigger" (RFC 8984, Section
// 4.5.1): an alert trigger that fires at a fixed [UTCDateTime].
type AbsoluteTrigger struct {
	// When is the absolute instant at which the alert fires (Section 4.5.1).
	When UTCDateTime
}

// absoluteTriggerWire is the JSON shape of an [AbsoluteTrigger]: the "@type"
// discriminator first, then the known members.
type absoluteTriggerWire struct {
	Type string      `json:"@type"`
	When UTCDateTime `json:"when"`
}

// triggerKind reports the trigger's "@type" discriminator value, here always
// "AbsoluteTrigger".
func (AbsoluteTrigger) triggerKind() string { return absoluteTriggerType }

// marshalTrigger encodes the AbsoluteTrigger with "@type" first, forcing the
// discriminator regardless of how the value was built.
func (a AbsoluteTrigger) marshalTrigger() ([]byte, error) {
	return json.Marshal(absoluteTriggerWire{
		Type: absoluteTriggerType,
		When: a.When,
	})
}

// triggerValue is the closed set of concrete alert-trigger kinds the library
// recognizes: [OffsetTrigger] and [AbsoluteTrigger]. An unrecognized kind is
// not a triggerValue — it is preserved as raw bytes on the enclosing
// [Trigger] instead, so the interface is deliberately unexported and closed.
type triggerValue interface {
	// triggerKind returns the value's "@type" discriminator.
	triggerKind() string
	// marshalTrigger encodes the value to its JSON object with "@type"
	// emitted first.
	marshalTrigger() ([]byte, error)
}

// Trigger is the polymorphic "trigger" member of an [Alert] (RFC 8984,
// Section 4.5.1). It holds one of the recognized concrete kinds —
// [OffsetTrigger] or [AbsoluteTrigger] — or, for a trigger whose "@type" the
// library does not recognize, the original JSON bytes so the trigger
// round-trips losslessly.
//
// Routing is on the wire "@type", never on a Go type assertion: decoding
// inspects the trigger object's "@type" member to choose a concrete kind, and
// falls back to byte preservation when it matches neither recognized value.
// Construct a Trigger with [NewTrigger]; read the concrete kind with a type
// switch over [Trigger.Value], which is nil for an unrecognized kind.
type Trigger struct {
	// value is the decoded concrete kind, or nil when the trigger's "@type"
	// is unrecognized (in which case raw holds the original bytes).
	value triggerValue
	// raw holds the original JSON bytes of an unrecognized trigger kind,
	// preserved verbatim so it round-trips. It is nil for a recognized kind.
	raw json.RawMessage
}

// NewTrigger wraps a recognized concrete trigger kind — an [OffsetTrigger] or
// [AbsoluteTrigger] — into a [Trigger] for assignment to [Alert.Trigger].
func NewTrigger[T interface {
	OffsetTrigger | AbsoluteTrigger
}](v T) Trigger {
	// The assertion cannot fail: the type constraint admits only
	// OffsetTrigger and AbsoluteTrigger, both of which satisfy triggerValue
	// (checked at compile time by the interface guards below).
	return Trigger{value: any(v).(triggerValue)}
}

// Compile-time guarantees that the recognized trigger kinds satisfy the
// triggerValue interface NewTrigger asserts to.
var (
	_ triggerValue = OffsetTrigger{}
	_ triggerValue = AbsoluteTrigger{}
)

// Value returns the decoded concrete trigger kind — an [OffsetTrigger] or
// [AbsoluteTrigger] — or nil when the trigger carried an "@type" the library
// does not recognize. A nil result with [Trigger.IsZero] false indicates an
// unrecognized kind preserved as raw bytes. Switch over the returned value's
// dynamic type to handle the recognized kinds.
func (t Trigger) Value() any {
	if t.value == nil {
		return nil
	}
	return t.value
}

// IsZero reports whether the Trigger holds neither a recognized kind nor
// preserved raw bytes — the zero value, which an [Alert] omits from its
// marshaled output.
func (t Trigger) IsZero() bool {
	return t.value == nil && t.raw == nil
}

// MarshalJSON encodes the Trigger. A recognized kind marshals through its own
// codec with "@type" first; an unrecognized kind re-emits the preserved raw
// bytes verbatim. The zero Trigger marshals as JSON null (it is omitted by
// the enclosing object via its omitempty / IsZero handling).
func (t Trigger) MarshalJSON() ([]byte, error) {
	switch {
	case t.value != nil:
		return t.value.marshalTrigger()
	case t.raw != nil:
		return t.raw, nil
	default:
		return []byte("null"), nil
	}
}

// UnmarshalJSON decodes a trigger object, routing on its wire "@type". An
// "OffsetTrigger" or "AbsoluteTrigger" decodes into the matching concrete
// kind; any other "@type" (or a missing one) is preserved as raw bytes so the
// trigger round-trips losslessly. A non-object input is a structural error.
func (t *Trigger) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*t = Trigger{}
		return nil
	}
	if !isJSONObject(data) {
		return errors.New("jscalendar: decode trigger: expected a JSON object")
	}

	var disc struct {
		Type string `json:"@type"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return fmt.Errorf("jscalendar: decode trigger: %w", err)
	}

	switch disc.Type {
	case offsetTriggerType:
		var w offsetTriggerWire
		if err := json.Unmarshal(data, &w); err != nil {
			return fmt.Errorf("jscalendar: decode OffsetTrigger: %w", err)
		}
		*t = Trigger{value: OffsetTrigger{Offset: w.Offset, RelativeTo: w.RelativeTo}}
	case absoluteTriggerType:
		var w absoluteTriggerWire
		if err := json.Unmarshal(data, &w); err != nil {
			return fmt.Errorf("jscalendar: decode AbsoluteTrigger: %w", err)
		}
		*t = Trigger{value: AbsoluteTrigger{When: w.When}}
	default:
		// Unrecognized trigger kind: preserve a copy of the original bytes
		// so it round-trips losslessly (the trigger "@type" registry is
		// open). The copy detaches from the decoder's buffer, which json
		// may reuse or mutate after UnmarshalJSON returns.
		*t = Trigger{raw: bytes.Clone(data)}
	}
	return nil
}
