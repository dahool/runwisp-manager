package chap

import "testing"

func TestResponse(t *testing.T) {
	tests := []struct{ name, password, nonce, want string }{{"password", "password", "nonce", "736b9e46edb32dbde0382c376f35dd8cf79b8338cff806a9cc2fccf588b08c44"}, {"hunter", "hunter2", "0123456789abcdef", "52e61d8f37e5294458ac2225126008ca1e32c7847b228221d574dc305f40f8ce"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Response(tt.password, tt.nonce); got != tt.want {
				t.Fatalf("response = %s, want %s", got, tt.want)
			}
		})
	}
}
