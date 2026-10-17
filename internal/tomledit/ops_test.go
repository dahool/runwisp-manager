package tomledit

import (
	"strings"
	"testing"
)

func TestStructuralOperations(t *testing.T) {
	src := []byte("[old]\na = 1\n\n[[jobs]]\nname = \"one\"\n\n[[jobs]]\nname = \"two\"\n")
	tests := []struct {
		name string
		op   Op
		want string
	}{
		{name: "add table", op: AddTable{Path: []string{"new"}, Body: []KV{{Key: "enabled", Value: true}}, Position: -1}, want: "[new]\nenabled = true"},
		{name: "rename table", op: RenameTable{From: []string{"old"}, To: []string{"renamed"}}, want: "[renamed]"},
		{name: "delete array table", op: DeleteTable{Path: []string{"jobs"}, Index: 0}, want: "name = \"two\""},
		{name: "reorder array tables", op: ReorderArrayTable{Path: []string{"jobs"}, From: 0, To: 1}, want: "name = \"two\"\n[[jobs]]\nname = \"one\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			out, err := doc.Apply([]Op{tt.op})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tt.want) {
				t.Fatalf("output %q lacks %q", out, tt.want)
			}
		})
	}
}
