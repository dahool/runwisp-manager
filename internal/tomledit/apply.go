package tomledit

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type edit struct {
	start, end int
	text       []byte
}

func (d *Doc) Apply(ops []Op) ([]byte, error) {
	if d == nil {
		return nil, errors.New("tomledit: nil document")
	}
	edits := make([]edit, 0, len(ops))
	for _, op := range ops {
		es, err := d.editsFor(op)
		if err != nil {
			return nil, err
		}
		edits = append(edits, es...)
	}
	out, err := splice(d.src, edits)
	if err != nil {
		return nil, err
	}
	if _, err := Parse(out); err != nil {
		return nil, fmt.Errorf("%w: output does not parse: %v", ErrUnsupportedLayout, err)
	}
	if err := semanticEqual(d.src, out, ops); err != nil {
		return nil, err
	}
	if err := locality(d.src, out, edits); err != nil {
		return nil, err
	}
	return append(bytes.Clone(d.bom), out...), nil
}

func (d *Doc) editsFor(op Op) ([]edit, error) {
	nl := "\n"
	if bytes.Contains(d.src, []byte("\r\n")) {
		nl = "\r\n"
	}
	switch x := op.(type) {
	case SetValue:
		n := d.find(x.Path)
		if n == nil || n.Kind != KindKeyValue {
			return nil, ErrNotFound
		}
		val, err := formatLike(x.Value, d.src[n.Value[0]:n.Value[1]], nl)
		if err != nil {
			return nil, err
		}
		if string(d.src[n.Value[0]:n.Value[1]]) == val {
			return nil, nil
		}
		return []edit{{n.Value[0], n.Value[1], []byte(val)}}, nil
	case DeleteKey:
		n := d.find(x.Path)
		if n == nil || n.Kind != KindKeyValue {
			return nil, ErrNotFound
		}
		return []edit{{n.Span[0], n.Span[1], nil}}, nil
	case AddKey:
		return d.addKey(x, nl)
	case AddTable:
		return d.addTable(x, nl)
	case DeleteTable:
		n := d.table(x.Path, x.Index)
		if n == nil {
			return nil, ErrNotFound
		}
		end := tableEnd(d, n)
		return []edit{{n.Span[0], end, nil}}, nil
	case RenameTable:
		n := d.table(x.From, 0)
		if n == nil {
			return nil, ErrNotFound
		}
		h := headerText(x.To, n.Kind == KindArrayTable)
		return []edit{{n.Header[0], n.Header[1], []byte(h)}}, nil
	case ReorderArrayTable:
		return d.reorder(x)
	default:
		return nil, fmt.Errorf("%w: unknown op %T", ErrUnsupportedLayout, op)
	}
}

func (d *Doc) find(path []string) *Node { return d.index[pathKey(path)] }
func (d *Doc) table(path []string, index int) *Node {
	count := 0
	for i := range d.nodes {
		n := &d.nodes[i]
		if (n.Kind == KindTable || n.Kind == KindArrayTable) && pathKey(n.Path) == pathKey(path) {
			if index == 0 || count == index {
				return n
			}
			count++
		}
	}
	return nil
}
func (d *Doc) addKey(x AddKey, nl string) ([]edit, error) {
	var keys []*Node
	for i := range d.nodes {
		n := &d.nodes[i]
		if n.Kind == KindKeyValue && pathKey(parent(n.Path)) == pathKey(x.Table) {
			keys = append(keys, n)
		}
	}
	if len(keys) == 0 {
		if t := d.table(x.Table, 0); t != nil {
			pos := t.Header[1]
			text := indentFor(d.src, t.Header[1]) + x.Key + " = "
			val, err := formatValue(x.Value, nl, false)
			if err != nil {
				return nil, err
			}
			return []edit{{pos, pos, []byte(nl + text + val + nl)}}, nil
		}
		if len(x.Table) == 0 {
			val, err := formatValue(x.Value, nl, false)
			if err != nil {
				return nil, err
			}
			return []edit{{len(d.src), len(d.src), []byte(x.Key + " = " + val + nl)}}, nil
		}
		return nil, ErrNotFound
	}
	pos := keys[len(keys)-1].Span[1]
	for _, k := range keys {
		if x.Before != "" && lastKey(k.Path) == x.Before {
			pos = k.Span[0]
		}
		if x.After != "" && lastKey(k.Path) == x.After {
			pos = k.Span[1]
		}
	}
	indent := indentFor(d.src, keys[len(keys)-1].Key[0])
	val, err := formatValue(x.Value, nl, false)
	if err != nil {
		return nil, err
	}
	text := indent + x.Key + " = " + val + nl
	return []edit{{pos, pos, []byte(text)}}, nil
}

func (d *Doc) addTable(x AddTable, nl string) ([]edit, error) {
	h := headerText(x.Path, x.Array)
	b := strings.Builder{}
	b.WriteString(h)
	for _, kv := range x.Body {
		v, err := formatValue(kv.Value, nl, false)
		if err != nil {
			return nil, err
		}
		b.WriteString(nl)
		b.WriteString(kv.Key)
		b.WriteString(" = ")
		b.WriteString(v)
	}
	b.WriteString(nl)
	pos := len(d.src)
	if pos > 0 && !bytes.HasSuffix(d.src, []byte(nl)) {
		b.WriteString(nl)
	}
	return []edit{{pos, pos, []byte(nl + b.String())}}, nil
}
func headerText(path []string, array bool) string {
	open, close := "[", "]"
	if array {
		open, close = "[[", "]]"
	}
	parts := make([]string, len(path))
	for i, p := range path {
		if strings.ContainsAny(p, " .\t\"") {
			parts[i] = strconvQuoteKey(p)
		} else {
			parts[i] = p
		}
	}
	return open + strings.Join(parts, ".") + close
}
func strconvQuoteKey(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
func parent(p []string) []string      { return p[:len(p)-1] }
func lastKey(p []string) string       { return p[len(p)-1] }
func indentFor(b []byte, pos int) string {
	line := lineStart(b, pos)
	i := line
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	return string(b[line:i])
}
func tableEnd(d *Doc, n *Node) int {
	end := len(d.src)
	for i := range d.nodes {
		x := &d.nodes[i]
		if (x.Kind == KindTable || x.Kind == KindArrayTable) && x.Header[0] > n.Header[0] && x.Span[0] < end {
			end = x.Span[0]
		}
	}
	return end
}
func (d *Doc) reorder(x ReorderArrayTable) ([]edit, error) {
	var nodes []*Node
	for i := range d.nodes {
		n := &d.nodes[i]
		if n.Kind == KindArrayTable && pathKey(n.Path) == pathKey(x.Path) {
			nodes = append(nodes, n)
		}
	}
	if x.From < 0 || x.From >= len(nodes) || x.To < 0 || x.To >= len(nodes) {
		return nil, ErrNotFound
	}
	a, b := nodes[x.From], nodes[x.To]
	aEnd, bEnd := tableEnd(d, a), tableEnd(d, b)
	text := bytes.Clone(d.src[a.Span[0]:aEnd])
	other := bytes.Clone(d.src[b.Span[0]:bEnd])
	if x.From < x.To {
		return []edit{{a.Span[0], bEnd, append(other, text...)}}, nil
	}
	return []edit{{b.Span[0], aEnd, append(text, other...)}}, nil
}

func splice(src []byte, edits []edit) ([]byte, error) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for i := 1; i < len(edits); i++ {
		if edits[i].end > edits[i-1].start {
			return nil, fmt.Errorf("%w: overlapping edits", ErrUnsupportedLayout)
		}
	}
	out := bytes.Clone(src)
	for _, e := range edits {
		out = append(append(append([]byte{}, out[:e.start]...), e.text...), out[e.end:]...)
	}
	return out, nil
}

func semanticEqual(before, after []byte, ops []Op) error {
	var a, b map[string]any
	if err := toml.Unmarshal(before, &a); err != nil {
		return err
	}
	if err := toml.Unmarshal(after, &b); err != nil {
		return err
	}
	if err := applyReference(a, ops); err != nil {
		return err
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("%w: expected %#v, got %#v", ErrSemanticMismatch, a, b)
	}
	return nil
}
func applyReference(root map[string]any, ops []Op) error {
	for _, raw := range ops {
		switch x := raw.(type) {
		case SetValue:
			setPath(root, x.Path, referenceValue(x.Value))
		case AddKey:
			setPath(root, append(clone(x.Table), x.Key), referenceValue(x.Value))
		case DeleteKey:
			deletePath(root, x.Path)
		case AddTable:
			m := map[string]any{}
			for _, kv := range x.Body {
				m[kv.Key] = referenceValue(kv.Value)
			}
			if x.Array {
				old, _ := getPath(root, x.Path).([]any)
				setPath(root, x.Path, append(old, m))
			} else {
				setPath(root, x.Path, m)
			}
		case RenameTable:
			v := getPath(root, x.From)
			deletePath(root, x.From)
			setPath(root, x.To, v)
		case DeleteTable:
			if values, ok := getPath(root, x.Path).([]any); ok && x.Index >= 0 && x.Index < len(values) {
				setPath(root, x.Path, append(values[:x.Index:x.Index], values[x.Index+1:]...))
			} else if x.Index == 0 {
				deletePath(root, x.Path)
			}
		case ReorderArrayTable:
			values, ok := getPath(root, x.Path).([]any)
			if !ok || x.From < 0 || x.From >= len(values) || x.To < 0 || x.To >= len(values) {
				return ErrNotFound
			}
			moved := values[x.From]
			values = append(values[:x.From:x.From], values[x.From+1:]...)
			values = append(values[:x.To], append([]any{moved}, values[x.To:]...)...)
			setPath(root, x.Path, values)
		}
	}
	return nil
}

func referenceValue(v any) any {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint:
		return int64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	case float32:
		return float64(x)
	case []string:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = referenceValue(x[i])
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = referenceValue(v)
		}
		return out
	default:
		return v
	}
}
func getPath(m map[string]any, p []string) any {
	var v any = m
	for _, k := range p {
		x, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = x[k]
	}
	return v
}
func setPath(m map[string]any, p []string, v any) {
	if len(p) == 0 {
		return
	}
	cur := m
	for _, k := range p[:len(p)-1] {
		n, ok := cur[k].(map[string]any)
		if !ok {
			n = map[string]any{}
			cur[k] = n
		}
		cur = n
	}
	cur[p[len(p)-1]] = v
}
func deletePath(m map[string]any, p []string) {
	if len(p) == 0 {
		return
	}
	cur := m
	for _, k := range p[:len(p)-1] {
		n, ok := cur[k].(map[string]any)
		if !ok {
			return
		}
		cur = n
	}
	delete(cur, p[len(p)-1])
}
func locality(before, after []byte, edits []edit) error {
	if len(edits) == 0 {
		if !bytes.Equal(before, after) {
			return fmt.Errorf("%w: changed bytes without an edit", ErrSemanticMismatch)
		}
		return nil
	}
	ordered := append([]edit(nil), edits...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].start < ordered[j].start })
	srcPos, dstPos := 0, 0
	for _, e := range ordered {
		unchanged := e.start - srcPos
		if unchanged < 0 || dstPos+unchanged > len(after) || !bytes.Equal(before[srcPos:e.start], after[dstPos:dstPos+unchanged]) {
			return fmt.Errorf("%w: bytes outside edit span changed", ErrSemanticMismatch)
		}
		srcPos = e.end
		dstPos += unchanged + len(e.text)
	}
	if srcPos > len(before) || dstPos > len(after) || !bytes.Equal(before[srcPos:], after[dstPos:]) {
		return fmt.Errorf("%w: bytes after edit span changed", ErrSemanticMismatch)
	}
	return nil
}
