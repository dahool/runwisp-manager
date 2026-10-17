package tomledit

import (
	"bytes"
	"fmt"
)

// Unit returns the source text owned by a key or table path. The returned bytes
// are independent of the document and may be modified by the caller.
func (d *Doc) Unit(path []string) ([]byte, bool) {
	if d == nil {
		return nil, false
	}
	n := d.find(path)
	if n == nil {
		return nil, false
	}
	start, end := n.Span[0], n.Span[1]
	if n.Kind == KindTable || n.Kind == KindArrayTable {
		end = tableEnd(d, n)
	}
	return append([]byte(nil), d.src[start:end]...), true
}

// ExtractUnit returns a unit and its half-open byte span. It returns a nil
// text and zero span when path is not indexed.
func (d *Doc) ExtractUnit(path []string) (text []byte, span [2]int) {
	if d == nil {
		return nil, span
	}
	n := d.find(path)
	if n == nil {
		return nil, span
	}
	span = n.Span
	if n.Kind == KindTable || n.Kind == KindArrayTable {
		span[1] = tableEnd(d, n)
	}
	return append([]byte(nil), d.src[span[0]:span[1]]...), span
}

// ReplaceUnit replaces exactly one owned unit and returns the complete
// document after applying the same parse safety check used by Apply.
func (d *Doc) ReplaceUnit(path []string, text []byte) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("tomledit: nil document")
	}
	_, span := d.ExtractUnit(path)
	if span[0] == span[1] {
		return nil, ErrNotFound
	}
	out, err := splice(d.src, []edit{{span[0], span[1], append([]byte(nil), text...)}})
	if err != nil {
		return nil, err
	}
	if _, err := Parse(out); err != nil {
		return nil, fmt.Errorf("%w: replacement does not parse: %v", ErrUnsupportedLayout, err)
	}
	return append(bytes.Clone(d.bom), out...), nil
}
