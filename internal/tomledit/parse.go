package tomledit

import (
	"bytes"
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Doc is an immutable parsed TOML document. The source is copied at Parse time.
type Doc struct {
	src   []byte
	bom   []byte
	nodes []Node
	index map[string]*Node
}

func Parse(src []byte) (*Doc, error) {
	var bom []byte
	if len(src) >= 3 && bytes.Equal(src[:3], []byte{0xef, 0xbb, 0xbf}) {
		bom = bytes.Clone(src[:3])
		src = src[3:]
	}
	data := bytes.Clone(src)
	d := &Doc{src: data, bom: bom, index: make(map[string]*Node)}
	p := new(unstable.Parser)
	p.KeepComments = true
	p.Reset(data)
	var table []string
	for p.NextExpression() {
		n := p.Expression()
		switch n.Kind {
		case unstable.Comment:
			start := int(n.Raw.Offset)
			end := start + int(n.Raw.Length)
			d.nodes = append(d.nodes, Node{Kind: KindComment, Span: [2]int{start, lineEnd(data, end)}})
		case unstable.Table, unstable.ArrayTable:
			path, keyRange, err := nodePath(n, p)
			if err != nil {
				return nil, err
			}
			table = path
			start, end := headerRange(data, keyRange, n.Kind == unstable.ArrayTable)
			span := ownedSpan(data, start, end, d.nodes)
			kind := KindTable
			if n.Kind == unstable.ArrayTable {
				kind = KindArrayTable
			}
			node := Node{Kind: kind, Path: clone(path), Span: span, Header: [2]int{start, end}}
			d.nodes = append(d.nodes, node)
		case unstable.KeyValue:
			path, keyRange, err := nodePath(n, p)
			if err != nil {
				return nil, err
			}
			value := n.Value().Raw
			ks, ke := int(keyRange.Offset), int(keyRange.Offset+keyRange.Length)
			vs, ve := int(value.Offset), int(value.Offset+value.Length)
			start := lineStart(data, ks)
			end := lineEnd(data, ve)
			span := ownedSpan(data, start, end, d.nodes)
			node := Node{Kind: KindKeyValue, Path: clone(append(clone(table), path...)), Span: span, Key: [2]int{ks, ke}, Value: [2]int{vs, ve}}
			d.nodes = append(d.nodes, node)
		}
	}
	if err := p.Error(); err != nil {
		return nil, err
	}
	if err := validateDuplicates(d); err != nil {
		return nil, err
	}
	for i := range d.nodes {
		n := &d.nodes[i]
		if n.Kind == KindKeyValue || n.Kind == KindTable || n.Kind == KindArrayTable {
			d.index[pathKey(n.Path)] = n
		}
	}
	return d, nil
}

func nodePath(n *unstable.Node, p *unstable.Parser) ([]string, unstable.Range, error) {
	var parts []string
	var first, last unstable.Range
	i := 0
	it := n.Key()
	for it.Next() {
		k := it.Node()
		if i == 0 {
			first = k.Raw
		}
		last = k.Raw
		parts = append(parts, decodeKey(p.Raw(k.Raw)))
		i++
	}
	if len(parts) == 0 {
		return nil, unstable.Range{}, fmt.Errorf("%w: empty key", ErrUnsupportedLayout)
	}
	lastEnd := last.Offset + last.Length
	first.Length = lastEnd - first.Offset
	return parts, first, nil
}

func decodeKey(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
		var value map[string]string
		if err := toml.Unmarshal([]byte("x = "+string(raw)), &value); err == nil {
			return value["x"]
		}
		return string(raw[1 : len(raw)-1])
	}
	return string(raw)
}

func pathKey(path []string) string { return strings.Join(path, "\x00") }
func clone(s []string) []string    { return append([]string(nil), s...) }
func lineStart(b []byte, pos int) int {
	if pos > len(b) {
		pos = len(b)
	}
	if i := bytes.LastIndexByte(b[:pos], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}
func lineEnd(b []byte, pos int) int {
	if pos > len(b) {
		pos = len(b)
	}
	if i := bytes.IndexByte(b[pos:], '\n'); i >= 0 {
		return pos + i + 1
	}
	return len(b)
}

func headerRange(b []byte, r unstable.Range, array bool) (int, int) {
	start := int(r.Offset)
	for start > 0 && (b[start-1] == ' ' || b[start-1] == '\t') {
		start--
	}
	for start > 0 && b[start-1] != '\n' && b[start-1] != '\r' {
		start--
	}
	end := int(r.Offset + r.Length)
	if i := bytes.IndexByte(b[end:], '\n'); i >= 0 {
		end += i
		if end > start && b[end-1] == '\r' {
			end--
		}
	} else {
		end = len(b)
	}
	return start, end
}

func ownedSpan(b []byte, start, end int, prior []Node) [2]int {
	line := lineStart(b, start)
	for line > 0 {
		prevEnd := line - 1
		prevStart := lineStart(b, prevEnd)
		text := bytes.TrimSpace(bytes.TrimSuffix(b[prevStart:prevEnd], []byte{'\r'}))
		if len(text) == 0 || text[0] != '#' {
			break
		}
		line = prevStart
	}
	return [2]int{line, end}
}

func validateDuplicates(d *Doc) error {
	seen := map[string]bool{}
	for _, n := range d.nodes {
		if n.Kind == KindTable {
			k := pathKey(n.Path)
			if seen[k] {
				return fmt.Errorf("%w: duplicate table %s", ErrUnsupportedLayout, strings.Join(n.Path, "."))
			}
			seen[k] = true
		}
	}
	return nil
}
