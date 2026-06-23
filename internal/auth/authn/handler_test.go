package authn

import "testing"

func TestIsValidRedirectPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid relative paths
		{name: "root", input: "/", want: true},
		{name: "simple path", input: "/dashboard", want: true},
		{name: "nested path", input: "/teams/my-team", want: true},
		{name: "path with query", input: "/path?query=value", want: true},
		{name: "path with query and fragment", input: "/path?query=value#fragment", want: true},
		{name: "trailing slash", input: "/path/with/trailing/slash/", want: true},

		// Empty or relative
		{name: "empty string", input: "", want: false},
		{name: "relative path", input: "relative/path", want: false},

		// Absolute URLs with scheme
		{name: "https", input: "https://example.com", want: false},
		{name: "https with path", input: "https://example.com/path", want: false},
		{name: "http", input: "http://example.com", want: false},
		{name: "javascript scheme", input: "javascript:alert(1)", want: false},
		{name: "data scheme", input: "data:text/html,<script>alert(1)</script>", want: false},

		// Protocol-relative and backslash tricks
		{name: "double slash", input: "//example.com", want: false},
		{name: "double slash with path", input: "//example.com/path", want: false},
		{name: "triple slash", input: "///example.com", want: false},
		{name: "quadruple slash", input: "////example.com", want: false},
		{name: "slash backslash", input: `/\example.com`, want: false},
		{name: "slash backslash slash", input: `/\/example.com`, want: false},
		{name: "slash backslash double", input: `/\/\/example.com`, want: false},
		{name: "backslash double slash", input: `\/\/example.com`, want: false},
		{name: "encoded backslash", input: "/%5Cexample.com", want: false},
		{name: "encoded backslash double", input: "/%5C/example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidRedirectPath(tt.input); got != tt.want {
				t.Errorf("isValidRedirectPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
