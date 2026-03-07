package service

import "testing"

func TestNormalizeAuthToken(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw token", input: "abc.def.ghi", want: "abc.def.ghi"},
		{name: "trim spaces", input: "  abc.def.ghi  ", want: "abc.def.ghi"},
		{name: "bearer token", input: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "bearer lowercase", input: "bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "bearer with extra spaces", input: "  Bearer   abc.def.ghi   ", want: "abc.def.ghi"},
		{name: "empty", input: "   ", want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeAuthToken(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeAuthToken(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

