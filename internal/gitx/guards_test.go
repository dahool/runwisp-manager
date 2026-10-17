package gitx

import "testing"

func TestContainsSecretCorpus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "password", input: "+password = \"literal\"", want: true},
		{name: "token", input: "+token = \"ghp_123456789012345678\"", want: true},
		{name: "private key", input: "+-----BEGIN PRIVATE KEY-----", want: true},
		{name: "environment reference", input: "+password = \"${PASSWORD}\"", want: false},
		{name: "file reference", input: "+token = \"${file:token}\"", want: false},
		{name: "lookalike", input: "+description = \"password policy\"", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsSecret(tt.input); got != tt.want {
				t.Fatalf("containsSecret(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
