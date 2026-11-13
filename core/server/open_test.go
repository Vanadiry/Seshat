package server

import "testing"

func TestParseHTTPURL(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"https://example.com/a?b=1", true},
		{"http://127.0.0.1:12500", true},
		{"  https://example.com  ", true},
		{"ftp://example.com", false},
		{"javascript:alert(1)", false},
		{"https://", false},
		{"", false},
		{"not a url", false},
	}
	for _, c := range cases {
		if _, ok := parseHTTPURL(c.in); ok != c.ok {
			t.Errorf("parseHTTPURL(%q) = %v，期望 %v", c.in, ok, c.ok)
		}
	}
}
