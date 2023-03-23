package shlex

import (
	"testing"
)

func TestJoin(t *testing.T) {
	// Test that Join returns the expected result for various inputs.
	cases := []struct {
		input    []string
		expected string
	}{
		{[]string{"echo", "hello world"}, "echo 'hello world'"},
		{[]string{"ls", "-l"}, "ls -l"},
		{[]string{"git", "commit", "-m", "fix: some bug"}, "git commit -m 'fix: some bug'"},
		{[]string{"echo", ""}, "echo ''"},
		{[]string{}, ""},
	}
	for _, c := range cases {
		actual := Join(c.input)
		if actual != c.expected {
			t.Errorf("Join(%v) == %q, expected %q", c.input, actual, c.expected)
		}
	}
}

func TestQuote(t *testing.T) {
	// Test that Quote returns the expected result for various inputs.
	cases := []struct {
		input    string
		expected string
	}{
		{"hello world", "'hello world'"},
		{"some/path/with spaces", "'some/path/with spaces'"},
		{"$PATH", "'$PATH'"},
		{"'", "''\"'\"''"},
		{"", "''"},
	}
	for _, c := range cases {
		actual := Quote(c.input)
		if actual != c.expected {
			t.Errorf("Quote(%q) == %q, expected %q", c.input, actual, c.expected)
		}
	}
}

func TestQuoteArg(t *testing.T) {
	// Test that QuoteArg returns the expected result for various inputs.
	cases := []struct {
		input    []string
		expected []string
	}{
		{[]string{"echo", "hello world"}, []string{"echo", "'hello world'"}},
		{[]string{"ls", "-l"}, []string{"ls", "-l"}},
		{[]string{"git", "commit", "-m", "fix: some bug"}, []string{"git", "commit", "-m", "'fix: some bug'"}},
		{[]string{"echo", ""}, []string{"echo", "''"}},
		{[]string{}, []string{}},
	}
	for _, c := range cases {
		actual := QuoteArg(c.input)
		if len(actual) != len(c.expected) {
			t.Errorf("QuoteArg(%v) == %v, expected %v", c.input, actual, c.expected)
		}
		for i := range actual {
			if actual[i] != c.expected[i] {
				t.Errorf("QuoteArg(%v)[%d] == %q, expected %q", c.input, i, actual[i], c.expected[i])
			}
		}
	}
}
