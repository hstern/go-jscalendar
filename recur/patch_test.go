// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur

import (
	"encoding/json"
	"strings"
	"testing"

	jscalendar "github.com/hstern/go-jscalendar"
)

// patchFrom builds a PatchObject from a JSON object literal, failing the test
// on a malformed literal.
func patchFrom(t *testing.T, jsonObj string) jscalendar.PatchObject {
	t.Helper()
	var p jscalendar.PatchObject
	if err := json.Unmarshal([]byte(jsonObj), &p); err != nil {
		t.Fatalf("decode patch %q: %v", jsonObj, err)
	}
	return p
}

// decodeObj decodes a JSON document into a generic object for field assertions.
func decodeObj(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return m
}

func TestApplyPatchSetsAndReplacesTopLevelMembers(t *testing.T) {
	doc := json.RawMessage(`{"@type":"Event","title":"Old","priority":3}`)
	patch := patchFrom(t, `{"title":"New","color":"blue"}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	m := decodeObj(t, got)
	if m["title"] != "New" {
		t.Errorf("title = %v, want New", m["title"])
	}
	if m["color"] != "blue" {
		t.Errorf("color = %v, want blue", m["color"])
	}
	if m["priority"].(float64) != 3 {
		t.Errorf("priority = %v, want 3 (untouched)", m["priority"])
	}
}

func TestApplyPatchSetsNestedPointer(t *testing.T) {
	doc := json.RawMessage(
		`{"@type":"Event","participants":{"abc":{"@type":"Participant","participationStatus":"accepted"}}}`,
	)
	patch := patchFrom(t, `{"participants/abc/participationStatus":"declined"}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	m := decodeObj(t, got)
	participants := m["participants"].(map[string]any)
	abc := participants["abc"].(map[string]any)
	if abc["participationStatus"] != "declined" {
		t.Errorf("participationStatus = %v, want declined", abc["participationStatus"])
	}
}

func TestApplyPatchNullRemovesMember(t *testing.T) {
	doc := json.RawMessage(`{"@type":"Event","title":"Keep","color":"red"}`)
	patch := patchFrom(t, `{"color":null}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	m := decodeObj(t, got)
	if _, ok := m["color"]; ok {
		t.Errorf("color should have been removed, got %v", m["color"])
	}
	if m["title"] != "Keep" {
		t.Errorf("title = %v, want Keep", m["title"])
	}
}

func TestApplyPatchNullOnAbsentMemberIsNoOp(t *testing.T) {
	doc := json.RawMessage(`{"@type":"Event","title":"Keep"}`)
	patch := patchFrom(t, `{"missing":null}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if m := decodeObj(t, got); m["title"] != "Keep" {
		t.Errorf("title = %v, want Keep", m["title"])
	}
}

func TestApplyPatchDecodesPointerEscapes(t *testing.T) {
	// "a/b" as a member name is escaped "a~1b"; "c~d" is "c~0d".
	doc := json.RawMessage(`{"a/b":{"c~d":1}}`)
	patch := patchFrom(t, `{"a~1b/c~0d":2}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	m := decodeObj(t, got)
	inner := m["a/b"].(map[string]any)
	if inner["c~d"].(float64) != 2 {
		t.Errorf("escaped pointer set = %v, want 2", inner["c~d"])
	}
}

func TestApplyPatchSetsArrayElement(t *testing.T) {
	doc := json.RawMessage(`{"comments":["one","two","three"]}`)
	patch := patchFrom(t, `{"comments/1":"TWO"}`)

	got, err := ApplyPatch(doc, patch)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	m := decodeObj(t, got)
	comments := m["comments"].([]any)
	if comments[1] != "TWO" {
		t.Errorf("comments[1] = %v, want TWO", comments[1])
	}
}

func TestApplyPatchErrors(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		patch string
		want  string
	}{
		{"absent parent", `{"@type":"Event"}`, `{"a/b/c":1}`, "parent member"},
		{"descend through scalar", `{"a":1}`, `{"a/b/c":2}`, "cannot descend"},
		{"array append", `{"a":[1]}`, `{"a/-":2}`, "array-append"},
		{"bad array index", `{"a":[1]}`, `{"a/x":2}`, "valid array index"},
		{"array index range", `{"a":[1]}`, `{"a/5":2}`, "out of range"},
		{"document not object", `[1,2]`, `{"a":1}`, "not a JSON object"},
		{"empty pointer", `{"a":1}`, `{"":1}`, "empty pointer"},
		{"remove array element", `{"a":[1,2]}`, `{"a/0":null}`, "unsupported"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ApplyPatch(json.RawMessage(tc.doc), patchFrom(t, tc.patch))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestApplyPatchEmptyPatchReturnsDocument(t *testing.T) {
	doc := json.RawMessage(`{"@type":"Event","title":"X"}`)
	got, err := ApplyPatch(doc, jscalendar.PatchObject{})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if m := decodeObj(t, got); m["title"] != "X" {
		t.Errorf("title = %v, want X", m["title"])
	}
}
