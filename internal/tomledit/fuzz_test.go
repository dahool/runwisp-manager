package tomledit

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte("name = \"runwisp\"\n[server]\nport = 8080\n"))
	f.Add([]byte("# comment\r\n[[jobs]]\r\nname = 'backup'\r\n"))
	f.Add([]byte("[a.b]\nc = [1, 2, 3]\n"))
	f.Fuzz(func(t *testing.T, src []byte) { _, _ = Parse(src) })
}

func FuzzApplySetValue(f *testing.F) {
	f.Add("hello", int64(1))
	f.Add("literal", int64(-42))
	f.Fuzz(func(t *testing.T, value string, number int64) {
		doc, err := Parse([]byte("name = \"old\"\ncount = 1\n"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := doc.Apply([]Op{SetValue{Path: []string{"name"}, Value: value}, SetValue{Path: []string{"count"}, Value: number}}); err != nil {
			// Errors are valid for values rejected by TOML; panics are not.
			t.Logf("clean edit rejection: %v", err)
		}
	})
}
