package load

import (
	"context"
	"testing"
)

func TestTargetsAreLoopbackOnly(t *testing.T) {
	for _, u := range []string{"https://127.0.0.1:8080", "http://example.com", "http://127.0.0.1.evil", "http://10.0.0.1"} {
		if _, e := Run(context.Background(), u, "x", "race", 1, 1); e == nil {
			t.Fatal(u)
		}
	}
}
func TestBounds(t *testing.T) {
	if _, e := Run(context.Background(), "http://127.0.0.1:8080", "x", "race", 0, 1); e == nil {
		t.Fatal("count accepted")
	}
}
