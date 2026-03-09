// Package lexver makes version strings comparable and sortable.
//
// Version numbers like "1.20.3" look numeric but aren't — as raw strings
// "1.2" > "1.20". Webi needs to find "the latest 1.20.x" or "the newest
// stable release" from a list. Lexver parses version strings into a struct
// and provides a comparison function for use with [slices.SortFunc].
//
// Pre-releases sort before their corresponding stable release:
//
//	1.0.0-alpha1 < 1.0.0-beta1 < 1.0.0-rc1 < 1.0.0
package lexver

import (
	"cmp"
	"strconv"
	"strings"
	"unicode"
)

// Version is a parsed version with comparable fields.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Channel    string // "" for stable, or "alpha", "beta", "dev", "pre", "preview", "rc"
	ChannelNum int    // e.g. 2 in "rc2"
	Date       string // release date "2024-01-15", if known (takes precedence over build)
	Raw        string // original string as provided
}

// Parse breaks a version string into its components.
func Parse(s string) Version {
	v := Version{Raw: s}

	s = strings.TrimLeft(s, "vV")

	numStr, prerelease := splitAtPrerelease(s)

	nums := splitNums(numStr)
	if len(nums) > 0 {
		v.Major = nums[0]
	}
	if len(nums) > 1 {
		v.Minor = nums[1]
	}
	if len(nums) > 2 {
		v.Patch = nums[2]
	}
	if prerelease != "" {
		v.Channel, v.ChannelNum = splitChannel(prerelease)
	}

	return v
}

// IsStable reports whether this is a stable (non-pre-release) version.
func (v Version) IsStable() bool {
	return v.Channel == ""
}

// Compare returns -1, 0, or 1 for ordering two versions.
// Stable releases sort after pre-releases of the same numeric version.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}

	// If both have dates, use them to break ties within the same
	// major.minor.patch. Dates are ISO strings so they compare correctly.
	if a.Date != "" && b.Date != "" {
		if c := cmp.Compare(a.Date, b.Date); c != 0 {
			return c
		}
	}

	// Both stable → equal (in version terms).
	if a.Channel == "" && b.Channel == "" {
		return 0
	}
	// Stable beats any pre-release.
	if a.Channel == "" {
		return 1
	}
	if b.Channel == "" {
		return -1
	}
	// Both pre-release: alphabetical channel, then number.
	if c := cmp.Compare(a.Channel, b.Channel); c != 0 {
		return c
	}
	return cmp.Compare(a.ChannelNum, b.ChannelNum)
}

// HasPrefix reports whether v matches a partial version prefix.
// A prefix matches if its non-zero fields equal the corresponding fields in v.
// For example, prefix {Major:1, Minor:20} matches any 1.20.x version.
func (v Version) HasPrefix(prefix Version) bool {
	if prefix.Major != v.Major {
		return false
	}
	if prefix.Minor != 0 && prefix.Minor != v.Minor {
		return false
	}
	if prefix.Patch != 0 && prefix.Patch != v.Patch {
		return false
	}
	return true
}

// splitAtPrerelease splits "1.20.3-beta1" into ("1.20.3", "beta1").
// Also handles "1.2beta3" (no separator).
func splitAtPrerelease(s string) (string, string) {
	for _, sep := range []byte{'-', '+'} {
		if idx := strings.IndexByte(s, sep); idx >= 0 {
			return s[:idx], s[idx+1:]
		}
	}

	// "1.2beta3": letter following a digit
	for i := 1; i < len(s); i++ {
		if unicode.IsLetter(rune(s[i])) && unicode.IsDigit(rune(s[i-1])) {
			return s[:i], s[i:]
		}
	}

	return s, ""
}

// splitNums parses "1.20.3" into [1, 20, 3].
func splitNums(s string) []int {
	var nums []int
	for _, seg := range strings.Split(s, ".") {
		n, err := strconv.Atoi(seg)
		if err != nil {
			break
		}
		nums = append(nums, n)
	}
	return nums
}

// splitChannel separates "beta1" into ("beta", 1) or "rc" into ("rc", 0).
func splitChannel(s string) (string, int) {
	s = strings.ToLower(s)
	s = strings.NewReplacer("-", "", ".", "", "_", "").Replace(s)

	i := len(s)
	for i > 0 && unicode.IsDigit(rune(s[i-1])) {
		i--
	}

	name := s[:i]
	num := 0
	if i < len(s) {
		num, _ = strconv.Atoi(s[i:])
	}

	return name, num
}
