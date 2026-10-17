package runwispbin

import "testing"

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name, version string
		supported     bool
	}{{"old", "1.6.9", false}, {"minimum", "1.7.0", true}, {"supported", "1.99.0", true}, {"major", "2.0.0", false}, {"invalid", "nope", false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckVersion(tt.version).Supported; got != tt.supported {
				t.Fatalf("supported = %v", got)
			}
		})
	}
}
