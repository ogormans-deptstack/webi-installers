package lexver_test

import (
	"slices"
	"testing"

	"github.com/webinstall/webi-installers/internal/lexver"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		major   int
		minor   int
		patch   int
		channel string
		chanNum int
	}{
		{"1.0.0", 1, 0, 0, "", 0},
		{"v1.2.3", 1, 2, 3, "", 0},
		{"1.20.156", 1, 20, 156, "", 0},
		{"1.20", 1, 20, 0, "", 0},
		{"1", 1, 0, 0, "", 0},
		{"1.0.0-beta1", 1, 0, 0, "beta", 1},
		{"1.0.0-rc2", 1, 0, 0, "rc", 2},
		{"2.0.0-alpha3", 2, 0, 0, "alpha", 3},
		{"1.0.0-dev", 1, 0, 0, "dev", 0},
		{"1.2beta3", 1, 2, 0, "beta", 3},
		{"1.0rc1", 1, 0, 0, "rc", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v := lexver.Parse(tt.input)
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("Parse(%q) = %d.%d.%d, want %d.%d.%d",
					tt.input, v.Major, v.Minor, v.Patch, tt.major, tt.minor, tt.patch)
			}
			if v.Channel != tt.channel || v.ChannelNum != tt.chanNum {
				t.Errorf("Parse(%q) channel = %q/%d, want %q/%d",
					tt.input, v.Channel, v.ChannelNum, tt.channel, tt.chanNum)
			}
		})
	}
}

func TestSortOrder(t *testing.T) {
	// Must be in ascending order.
	ordered := []string{
		"0.1.0",
		"1.0.0-alpha1",
		"1.0.0-alpha2",
		"1.0.0-beta1",
		"1.0.0-rc1",
		"1.0.0-rc2",
		"1.0.0",
		"1.0.1",
		"1.1.0",
		"1.2.0",
		"1.20.0",
		"2.0.0-beta1",
		"2.0.0",
	}

	for i := 1; i < len(ordered); i++ {
		a := lexver.Parse(ordered[i-1])
		b := lexver.Parse(ordered[i])
		if lexver.Compare(a, b) >= 0 {
			t.Errorf("expected %q < %q", ordered[i-1], ordered[i])
		}
	}
}

func TestSortFunc(t *testing.T) {
	versions := []string{"1.0.0", "2.0.0-rc1", "1.20.3", "1.20.2", "1.19.5", "2.0.0"}
	parsed := make([]lexver.Version, len(versions))
	for i, s := range versions {
		parsed[i] = lexver.Parse(s)
	}

	// Sort descending (newest first).
	slices.SortFunc(parsed, func(a, b lexver.Version) int {
		return lexver.Compare(b, a)
	})

	want := []string{"2.0.0", "2.0.0-rc1", "1.20.3", "1.20.2", "1.19.5", "1.0.0"}
	for i, v := range parsed {
		if v.Raw != want[i] {
			t.Errorf("index %d: got %q, want %q", i, v.Raw, want[i])
		}
	}
}

func TestIsStable(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1.0.0", true},
		{"1.0.0-beta1", false},
		{"1.0.0-rc2", false},
		{"v2.0.0-dev", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v := lexver.Parse(tt.input)
			if v.IsStable() != tt.want {
				t.Errorf("Parse(%q).IsStable() = %v, want %v", tt.input, v.IsStable(), tt.want)
			}
		})
	}
}

func TestDateTiebreaker(t *testing.T) {
	a := lexver.Parse("1.0.0")
	a.Date = "2024-01-15"

	b := lexver.Parse("1.0.0")
	b.Date = "2024-06-01"

	if lexver.Compare(a, b) >= 0 {
		t.Error("earlier date should sort before later date at same version")
	}

	// Without dates, same version is equal.
	c := lexver.Parse("1.0.0")
	d := lexver.Parse("1.0.0")
	if lexver.Compare(c, d) != 0 {
		t.Error("same version without dates should be equal")
	}

	// Date only matters when both have it.
	e := lexver.Parse("1.0.0")
	e.Date = "2024-01-15"
	f := lexver.Parse("1.0.0")
	if lexver.Compare(e, f) != 0 {
		t.Error("date should be ignored when only one side has it")
	}
}

func TestHasPrefix(t *testing.T) {
	v := lexver.Parse("1.20.3")

	if !v.HasPrefix(lexver.Version{Major: 1, Minor: 20}) {
		t.Error("1.20.3 should match prefix 1.20")
	}
	if !v.HasPrefix(lexver.Version{Major: 1}) {
		t.Error("1.20.3 should match prefix 1")
	}
	if v.HasPrefix(lexver.Version{Major: 1, Minor: 19}) {
		t.Error("1.20.3 should not match prefix 1.19")
	}
	if v.HasPrefix(lexver.Version{Major: 2}) {
		t.Error("1.20.3 should not match prefix 2")
	}
}
