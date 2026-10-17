package tomledit

// Source returns a defensive copy of the parsed source, including its BOM.
func (d *Doc) Source() []byte {
	if d == nil {
		return nil
	}
	out := make([]byte, 0, len(d.bom)+len(d.src))
	out = append(out, d.bom...)
	return append(out, d.src...)
}

// Nodes returns a defensive copy of the source index.
func (d *Doc) Nodes() []Node {
	if d == nil {
		return nil
	}
	out := make([]Node, len(d.nodes))
	copy(out, d.nodes)
	for i := range out {
		out[i].Path = clone(out[i].Path)
	}
	return out
}
