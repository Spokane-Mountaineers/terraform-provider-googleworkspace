package client

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "all empty", in: []string{"", ""}, want: ""},
		{name: "first wins", in: []string{"a", "b"}, want: "a"},
		{name: "skips empty", in: []string{"", "b", "c"}, want: "b"},
		{name: "none", in: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := firstNonEmpty(tt.in...); got != tt.want {
				t.Fatalf("firstNonEmpty(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
