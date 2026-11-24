package survey

import (
	"strings"
	"testing"
)

func testStringTransformer(t *testing.T, f func(string) string) {
	transformer := TransformString(f)

	tests := []string{
		"hello my name is",
		"where are you from",
		"does that matter?",
	}

	for _, tt := range tests {
		if expected, got := f(tt), transformer(tt); expected != got {
			t.Errorf("TransformString transformer failed to transform the answer, expected '%s' but got '%s'.", expected, got)
		}
	}
}

func TestTransformString(t *testing.T) {
	testStringTransformer(t, strings.ToTitle) // all letters titled
	testStringTransformer(t, strings.ToLower) // all letters lowercase
}

func TestComposeTransformers(t *testing.T) {
	// create a transformer which makes no sense,
	// remember: transformer can be used for any type
	// we just test the built'n functions that
	// happens to be for strings only.
	transformer := ComposeTransformers(
		Title,
		ToLower,
	)

	ans := "my name is"
	if expected, got := strings.ToLower(ans), transformer(ans); expected != got {
		// the result should be lowercase.
		t.Errorf("TestComposeTransformers transformer failed to transform the answer to title->lowercase, expected '%s' but got '%s'.", expected, got)
	}

	var emptyAns string
	if expected, got := "", transformer(emptyAns); expected != got {
		// TransformString transformers should be skipped and return zero value string
		t.Errorf("TestComposeTransformers transformer failed to skip transforming on optional empty input, expected '%s' but got '%s'.", expected, got)
	}
}

// TestTitle pins the title casing that Title delegates to
// golang.org/x/text/cases. The dependency is meant to be upgraded over time,
// and its word boundary rules for apostrophes, ligatures and non-latin
// scripts have changed between releases before. These expectations are the
// observable contract of the exported transformer, so a dependency bump that
// alters them has to be a deliberate, reviewed decision.
func TestTitle(t *testing.T) {
	tests := map[string]string{
		"hello world":         "Hello World",
		"HELLO WORLD":         "Hello World",
		"don't stop":          "Don't Stop",
		"it's fine":           "It's Fine",
		"o'neill":             "O'neill",
		"ǆungla":              "ǅungla",
		"ﬁnal":                "Final",
		"İstanbul":            "İstanbul",
		"ß":                   "Ss",
		"éclair":              "Éclair",
		"ΓΛΩΣΣΑ":              "Γλωσσα",
		"hello-world foo_bar": "Hello-World Foo_bar",
		"  spaced  ":          "  Spaced  ",
		"123abc":              "123Abc",
		"":                    "",
	}

	for in, expected := range tests {
		if got := Title(in); got != expected {
			t.Errorf("Title(%q) = %q, expected %q", in, got, expected)
		}
	}

	// Non-string answers become the zero value string.
	if got := Title(42); got != "" {
		t.Errorf("Title(non-string) = %q, expected an empty string", got)
	}
}
