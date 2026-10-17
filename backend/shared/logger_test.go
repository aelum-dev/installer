package shared

import "testing"

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"double quote", `say "hi"`, `say \"hi\"`},
		{"backslash", `C:\path\to`, `C:\\path\\to`},
		{"newline", "line1\nline2", `line1\nline2`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"mixed", "a\"b\\c\nd", `a\"b\\c\nd`},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeJSON(tt.in); got != tt.want {
				t.Errorf("escapeJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestEscapeJSON_BackslashBeforeQuote is a regression guard for the ordering
// of replacements: backslashes must be escaped before quotes, otherwise the
// backslash introduced by escaping a quote would itself get doubled.
func TestEscapeJSON_BackslashBeforeQuote(t *testing.T) {
	// Input is the two characters: backslash, quote.
	if got := escapeJSON("\\\""); got != `\\\"` {
		t.Errorf("escapeJSON(%q) = %q, want %q", "\\\"", got, `\\\"`)
	}
}
