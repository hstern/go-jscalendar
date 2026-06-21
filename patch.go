// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// PatchObject is a JSCalendar PatchObject (RFC 8984 §1.4.9, §3.3): a map
// from JSON Pointer (RFC 6901) reference to the replacement value at that
// location in the object being patched.
//
// It is the wire shape behind [recurrenceOverrides] and [localizations].
// Each key is a JSON Pointer *relative to the patched object* — that is,
// it omits the leading "/" that an absolute RFC 6901 pointer carries
// (§1.4.9). Each value is the JSON to set at that location, retained as
// [json.RawMessage] so the exact bytes round-trip: interop scenarios pin
// the value byte-for-byte, and a map[string]any would reorder nested
// keys.
//
// A value of JSON null is special: it means *remove* the property the
// pointer addresses, rather than set it to null (§3.3). Use [IsRemoval]
// to distinguish a removal from a literal value, and [Removals] to list
// the removed pointers.
//
// Per Postel's law the codec is lenient on decode (any JSON object
// decodes) and the bytes are preserved verbatim; semantic constraints —
// pointer well-formedness and the forbidden root "@type" target — are
// checked only by the opt-in [PatchObject.Validate]. Applying a patch
// against a concrete document (including the §3.3 rule that a patch must
// not create a property whose parent is absent) is a higher-layer
// concern and is not performed here.
//
// [recurrenceOverrides]: https://www.rfc-editor.org/rfc/rfc8984.html#section-4.3.5
// [localizations]: https://www.rfc-editor.org/rfc/rfc8984.html#section-4.6.1
type PatchObject map[string]json.RawMessage

// MarshalJSON encodes the patch as a JSON object with keys in sorted
// order, yielding byte-stable output suitable for embedding in a
// byte-stable parent document.
//
// A nil PatchObject encodes as an empty object ("{}"), never as JSON
// null: a PatchObject that is present on its parent is always a value.
// Each retained value is emitted verbatim, so a value of "null" appears
// as a JSON null (the §3.3 removal sentinel) rather than being dropped.
func (p PatchObject) MarshalJSON() ([]byte, error) {
	if len(p) == 0 {
		return []byte("{}"), nil
	}

	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, fmt.Errorf("jscalendar: marshal patch key %q: %w", k, err)
		}
		buf.Write(key)
		buf.WriteByte(':')

		// json.Compact validates the value as well-formed JSON and
		// strips insignificant whitespace, keeping the output canonical
		// without reordering any nested object keys.
		val := p[k]
		if err := json.Compact(&buf, val); err != nil {
			return nil, fmt.Errorf("jscalendar: marshal patch value at %q: %w", k, err)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON decodes a JSON object into the patch, retaining each
// value as raw bytes. It is lenient: any JSON object is accepted, and no
// pointer or target validation is performed (that is [PatchObject.Validate]'s
// job). A non-object input — array, scalar, or null — is an error, since a
// PatchObject is always an object.
func (p *PatchObject) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("jscalendar: decode patch object: %w", err)
	}
	if raw == nil {
		// Input was JSON null. A PatchObject is never null on the wire.
		return fmt.Errorf("jscalendar: decode patch object: unexpected null")
	}
	*p = raw
	return nil
}

// IsRemoval reports whether the value at key is the JSON null removal
// sentinel (RFC 8984 §3.3). It returns false for a missing key and for
// any non-null value.
func (p PatchObject) IsRemoval(key string) bool {
	v, ok := p[key]
	if !ok {
		return false
	}
	return isJSONNull(v)
}

// Removals returns the pointer keys whose value is the JSON null removal
// sentinel, in sorted order.
func (p PatchObject) Removals() []string {
	var out []string
	for k, v := range p {
		if isJSONNull(v) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// Validate checks every key for JSON Pointer well-formedness and the
// JSCalendar PatchObject key rules (RFC 8984 §1.4.9): relative form (no
// leading slash), non-empty, and never targeting the root "@type". Keys
// are checked in sorted order so the first reported error is
// deterministic. It returns a [*PatchError] on the first offending key,
// or nil if every key is well-formed.
//
// Validate does not resolve pointers against any document, so it does not
// enforce the §3.3 "parent must exist" rule — that requires the target
// object and belongs to the patch-application layer.
func (p PatchObject) Validate() error {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := ValidatePatchKey(k); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePatchKey reports whether key is a valid JSCalendar PatchObject
// key: a JSON Pointer relative to the patched object (RFC 8984 §1.4.9).
// A valid key is non-empty, carries no leading "/", is well-formed per
// RFC 6901 once the implicit leading slash is restored, and does not
// address the root "@type" member. Only the *root* "@type" is forbidden;
// a nested "@type" (e.g. "locations/1/@type") is a legitimate target.
func ValidatePatchKey(key string) error {
	if key == "" {
		return &PatchError{Pointer: key, Reason: "key must not be empty"}
	}
	if strings.HasPrefix(key, "/") {
		return &PatchError{
			Pointer: key,
			Reason:  "key is relative to the patched object and must not start with '/'",
		}
	}
	if key == "@type" {
		return &PatchError{Pointer: key, Reason: "must not target the root @type"}
	}
	// Validate as an absolute RFC 6901 pointer by restoring the implicit
	// leading slash that the relative JSCalendar form omits.
	if err := ValidatePointer("/" + key); err != nil {
		return err
	}
	return nil
}

// ValidatePointer reports whether ptr is a well-formed JSON Pointer per
// RFC 6901: either the empty string (the whole-document pointer) or a
// string in which every "~" is part of a "~0" or "~1" escape. It does not
// require a leading "/" beyond what RFC 6901 mandates for non-empty
// pointers, and it does not resolve the pointer against any document.
//
// JSCalendar PatchObject keys use the relative form (no leading slash);
// validate those with [ValidatePatchKey], which adds the JSCalendar key
// rules on top of this RFC 6901 check.
func ValidatePointer(ptr string) error {
	if ptr == "" {
		return nil
	}
	if !strings.HasPrefix(ptr, "/") {
		return &PatchError{
			Pointer: ptr,
			Reason:  "non-empty JSON Pointer must start with '/'",
		}
	}
	for i := 0; i < len(ptr); i++ {
		if ptr[i] != '~' {
			continue
		}
		if i+1 >= len(ptr) || (ptr[i+1] != '0' && ptr[i+1] != '1') {
			return &PatchError{
				Pointer: ptr,
				Reason:  "'~' must be escaped as '~0' or '~1'",
			}
		}
		i++ // skip the escape digit
	}
	return nil
}

// PatchError reports a malformed or disallowed PatchObject pointer. It
// names the offending Pointer (the key as it appeared in the patch) and
// a human-readable Reason.
type PatchError struct {
	// Pointer is the offending key, as it appeared in the PatchObject.
	Pointer string
	// Reason explains why the pointer is invalid.
	Reason string
}

// Error implements the error interface.
func (e *PatchError) Error() string {
	return fmt.Sprintf("jscalendar: invalid patch pointer %q: %s", e.Pointer, e.Reason)
}

// isJSONNull reports whether raw is the JSON null literal, ignoring any
// surrounding insignificant whitespace.
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
