package main

import (
	"strings"
	"testing"
)

func TestNormalizeJSONSortsKeysAndFormats(t *testing.T) {
	input := []byte(`{"z":1,"a":{"y":2,"x":3}}`)
	got, err := normalizeJSON(input)
	if err != nil {
		t.Fatalf("normalizeJSON returned error: %v", err)
	}

	want := "{\n  \"a\": {\n    \"x\": 3,\n    \"y\": 2\n  },\n  \"z\": 1\n}"
	if got != want {
		t.Fatalf("unexpected normalized JSON\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestNormalizeJSONRejectsTrailingValue(t *testing.T) {
	_, err := normalizeJSON([]byte(`{"a":1} {"b":2}`))
	if err == nil {
		t.Fatal("normalizeJSON should reject multiple JSON values")
	}
}

func TestBuildRowsCountsAndKinds(t *testing.T) {
	before := "A\nB\nC\n"
	after := "A\nX\nC\nD\n"

	rows, add, del, changed := buildRows(before, after)
	if len(rows) == 0 {
		t.Fatal("rows should not be empty")
	}
	if add != 1 {
		t.Fatalf("add count mismatch: got %d want 1", add)
	}
	if del != 0 {
		t.Fatalf("delete count mismatch: got %d want 0", del)
	}
	if changed != 1 {
		t.Fatalf("changed count mismatch: got %d want 1", changed)
	}

	kinds := []string{rows[0].Kind, rows[1].Kind, rows[2].Kind, rows[3].Kind}
	want := []string{"equal", "changed", "equal", "added"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("row kind mismatch at %d: got %s want %s", i, kinds[i], want[i])
		}
	}
}

func TestBuildRowsHandlesInsertBeforeDeleteBlock(t *testing.T) {
	before := "A\nB\nC\n"
	after := "A\nX\nY\nC\n"

	rows, add, del, changed := buildRows(before, after)
	if add != 1 {
		t.Fatalf("add count mismatch: got %d want 1", add)
	}
	if del != 0 {
		t.Fatalf("delete count mismatch: got %d want 0", del)
	}
	if changed != 1 {
		t.Fatalf("changed count mismatch: got %d want 1", changed)
	}
	if rows[1].Kind != "changed" || rows[2].Kind != "added" {
		t.Fatalf("unexpected row kinds around change: got %s, %s", rows[1].Kind, rows[2].Kind)
	}
}

func TestSplitLinesKeepsBlankLines(t *testing.T) {
	got := splitLines("A\n\nB\n")
	want := []string{"A", "", "B"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("splitLines mismatch: got %#v want %#v", got, want)
	}
}

func TestInlineHighlightMarksInsertAndDelete(t *testing.T) {
	left, right := inlineHighlight("abc", "adc")
	if !strings.Contains(string(left), "inline-del") {
		t.Fatal("left side should contain delete highlight")
	}
	if !strings.Contains(string(right), "inline-ins") {
		t.Fatal("right side should contain insert highlight")
	}
}
