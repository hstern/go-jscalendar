// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	jscalendar "github.com/hstern/go-jscalendar"
)

// ApplyPatch applies a JSCalendar [jscalendar.PatchObject] to a JSON document
// and returns the patched document (RFC 8984, Section 3.3). It is the "apply"
// half of the PatchObject codec — the layer the core jscalendar package
// deliberately leaves out, since applying a patch needs a concrete target
// document, not just the patch's wire bytes.
//
// doc must encode a JSON object. Each patch key is a JSON Pointer (RFC 6901)
// relative to that object, with the leading "/" omitted per JSCalendar's
// relative-pointer convention (Section 1.4.9); "~1" and "~0" escapes decode to
// "/" and "~". For each key:
//
//   - A value of JSON null removes the member the pointer addresses (the
//     Section 3.3 removal sentinel). Removing an absent member is a no-op.
//   - Any other value is set at the pointer, replacing an existing member or
//     adding a new one. The member's parent container must already exist
//     (Section 3.3): a patch may add a leaf but not conjure intermediate
//     objects. A key whose parent is absent is an error.
//
// Keys are applied in lexicographic order, so a key that creates a container
// (for example "locations") is applied before one that targets inside it
// ("locations/x/name"). Object members are addressed by name and array
// elements by decimal index; the array-append token "-" and removal of an
// array element are not supported and return an error, as JSCalendar patches
// address object members in practice.
//
// The returned document is freshly marshaled and is not byte-stable with
// respect to doc — re-decode it into the relevant jscalendar type for a
// byte-stable round trip.
func ApplyPatch(doc json.RawMessage, patch jscalendar.PatchObject) (json.RawMessage, error) {
	var root any
	if err := json.Unmarshal(doc, &root); err != nil {
		return nil, fmt.Errorf("recur: apply patch: decode document: %w", err)
	}
	if _, ok := root.(map[string]any); !ok {
		return nil, fmt.Errorf("recur: apply patch: document is not a JSON object")
	}

	keys := make([]string, 0, len(patch))
	for k := range patch {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, key := range keys {
		raw := patch[key]
		if isJSONNull(raw) {
			if err := removePointer(root, key); err != nil {
				return nil, err
			}
			continue
		}
		var val any
		if err := json.Unmarshal(raw, &val); err != nil {
			return nil, fmt.Errorf("recur: apply patch: decode value at %q: %w", key, err)
		}
		if err := setPointer(root, key, val); err != nil {
			return nil, err
		}
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("recur: apply patch: re-encode document: %w", err)
	}
	return out, nil
}

// patchTokens splits a relative JSCalendar pointer into its decoded reference
// tokens, undoing the RFC 6901 "~1" → "/" and "~0" → "~" escapes (in that
// order). The empty key — which would address the whole document — yields no
// tokens and is rejected by the callers.
func patchTokens(key string) []string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		parts[i] = p
	}
	return parts
}

// setPointer sets val at the location key addresses within root, walking the
// pointer one token at a time. Every token but the last must name an existing
// container (an object member or array element); the last token names the
// member to set, which may be new on an existing object.
func setPointer(root any, key string, val any) error {
	tokens := patchTokens(key)
	parent, last, err := navigate(root, tokens, key)
	if err != nil {
		return err
	}
	switch c := parent.(type) {
	case map[string]any:
		c[last] = val
		return nil
	case []any:
		i, err := arrayIndex(last, len(c), key)
		if err != nil {
			return err
		}
		c[i] = val
		return nil
	default:
		return fmt.Errorf("recur: apply patch: cannot set %q: parent is not a container", key)
	}
}

// removePointer deletes the member key addresses within root. Removing an
// absent object member is a no-op; removing an array element is unsupported.
func removePointer(root any, key string) error {
	tokens := patchTokens(key)
	parent, last, err := navigate(root, tokens, key)
	if err != nil {
		return err
	}
	switch c := parent.(type) {
	case map[string]any:
		delete(c, last)
		return nil
	case []any:
		return fmt.Errorf("recur: apply patch: cannot remove array element %q: unsupported", key)
	default:
		return fmt.Errorf("recur: apply patch: cannot remove %q: parent is not a container", key)
	}
}

// navigate walks all but the final token of a pointer and returns the
// container that holds the final token together with that token. key is passed
// only for error messages.
func navigate(root any, tokens []string, key string) (parent any, last string, err error) {
	if len(tokens) == 0 || (len(tokens) == 1 && tokens[0] == "") {
		return nil, "", fmt.Errorf("recur: apply patch: empty pointer %q", key)
	}
	cur := root
	for _, tok := range tokens[:len(tokens)-1] {
		switch c := cur.(type) {
		case map[string]any:
			next, ok := c[tok]
			if !ok {
				return nil, "", fmt.Errorf("recur: apply patch: %q: parent member %q is absent", key, tok)
			}
			cur = next
		case []any:
			i, err := arrayIndex(tok, len(c), key)
			if err != nil {
				return nil, "", err
			}
			cur = c[i]
		default:
			return nil, "", fmt.Errorf("recur: apply patch: %q: cannot descend through %q", key, tok)
		}
	}
	return cur, tokens[len(tokens)-1], nil
}

// arrayIndex parses an RFC 6901 array index token and bounds-checks it against
// length. The append token "-" is rejected: JSCalendar patches address
// existing members, and growing an array through a patch is out of scope.
func arrayIndex(tok string, length int, key string) (int, error) {
	if tok == "-" {
		return 0, fmt.Errorf("recur: apply patch: %q: array-append token %q is unsupported", key, tok)
	}
	i, ok := parseIndex(tok)
	if !ok {
		return 0, fmt.Errorf("recur: apply patch: %q: %q is not a valid array index", key, tok)
	}
	if i < 0 || i >= length {
		return 0, fmt.Errorf("recur: apply patch: %q: array index %d out of range", key, i)
	}
	return i, nil
}

// parseIndex parses a non-negative decimal array index with no leading zeros
// (RFC 6901), reporting ok=false for anything else.
func parseIndex(tok string) (int, bool) {
	if tok == "" || (len(tok) > 1 && tok[0] == '0') {
		return 0, false
	}
	n := 0
	for i := 0; i < len(tok); i++ {
		if tok[i] < '0' || tok[i] > '9' {
			return 0, false
		}
		n = n*10 + int(tok[i]-'0')
	}
	return n, true
}

// isJSONNull reports whether raw is the JSON null literal, ignoring
// surrounding insignificant whitespace.
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
