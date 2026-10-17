package tomledit

import (
	"bytes"
	"testing"
)

func TestExtractAndReplaceUnitRoundTrip(t *testing.T) {
	src := []byte("# docs\n[server]\nport = 8080\n\n[other]\nvalue = true\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	unit, span := doc.ExtractUnit([]string{"server"})
	if span[1] <= span[0] || !bytes.Contains(unit, []byte("port = 8080")) {
		t.Fatalf("bad unit %q span %#v", unit, span)
	}
	out, err := doc.ReplaceUnit([]string{"server"}, unit)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("round trip changed source: %q", out)
	}
	if got, ok := doc.Unit([]string{"missing"}); ok || got != nil {
		t.Fatalf("missing unit returned %q", got)
	}
}
