package gedcom

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseAndWrite(t *testing.T) {
	src := "\uFEFF0 HEAD\r\n1 CHAR UTF-8\r\n0 @I1@ INDI\r\n1 NAME Ann /Lee/\r\n1 NOTE line one\r\n2 CONT line two\r\n2 CONC  continued\r\n0 TRLR\r\n"
	roots, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 3 || roots[1].Xref != "@I1@" || roots[1].Tag != "INDI" {
		t.Fatalf("roots: %+v", roots)
	}
	if got := roots[1].Child("NOTE").Text(); got != "line one\nline two continued" {
		t.Fatalf("note text %q", got)
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Line(1, "", "NOTE", "a\nb"+strings.Repeat("z", 450))
	w.Flush()
	again, err := Parse(strings.NewReader("0 HEAD\n0 @X@ INDI\n" + buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got := again[1].Child("NOTE").Text(); got != "a\nb"+strings.Repeat("z", 450) {
		t.Fatalf("round trip lost data: %d chars", len(got))
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{"", "hello world", "0 INDI\n", "0 HEAD\n2 SKIP level\n", "0 HEAD\nX TAG\n"} {
		if _, err := Parse(strings.NewReader(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}
