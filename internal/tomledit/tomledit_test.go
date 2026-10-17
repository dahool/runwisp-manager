package tomledit

import (
	"bytes"
	"errors"
	"testing"
)

func TestParseAndSetValuePreservesLocality(t *testing.T) {
	src := []byte("# keep\nname = \"old\" # trailing\n\n[server]\n# port docs\nport = 8080\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Apply([]Op{SetValue{Path: []string{"server", "port"}, Value: 9090}})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("# keep\nname = \"old\" # trailing\n\n[server]\n# port docs\nport = 9090\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestAddAndDeleteKey(t *testing.T) {
	src := []byte("[server]\na = 1\nb = 2\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Apply([]Op{AddKey{Table: []string{"server"}, Key: "c", Value: true}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[server]\na = 1\nb = 2\nc = true\n" {
		t.Fatalf("add: %q", out)
	}
	doc, err = Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	out, err = doc.Apply([]Op{DeleteKey{Path: []string{"server", "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[server]\na = 1\nc = true\n" {
		t.Fatalf("delete: %q", out)
	}
}

func TestParsePreservesBOMAndCRLFOnInsert(t *testing.T) {
	src := []byte("\xef\xbb\xbf[server]\r\na = 1\r\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Apply([]Op{AddKey{Table: []string{"server"}, Key: "b", Value: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("\r\n")) || !bytes.HasPrefix(out, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("unexpected line ending/BOM handling: %q", out)
	}
}

func TestIdempotentSetValue(t *testing.T) {
	src := []byte("x = 1\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Apply([]Op{SetValue{Path: []string{"x"}, Value: int64(1)}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("changed unchanged value: %q", out)
	}
}

func TestUnsupportedAndNotFoundErrors(t *testing.T) {
	doc, err := Parse([]byte("x = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = doc.Apply([]Op{SetValue{Path: []string{"missing"}, Value: 2}})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
