package version

import "testing"

func TestString_defaults(t *testing.T) {
	// With the default ldflags values the package vars are "dev", "none", "unknown".
	want := "dev (none) unknown"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestString_custom_values(t *testing.T) {
	origV, origC, origD := Version, Commit, Date
	defer func() { Version, Commit, Date = origV, origC, origD }()

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2025-01-15"

	want := "1.2.3 (abc1234) 2025-01-15"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
