//
// Copyright: (C) 2020 Nestybox Inc.  All rights reserved.
//

package utils

import (
	"github.com/opencontainers/runtime-spec/specs-go"
)

// StringSliceContains returns true if x is in a
func StringSliceContains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

// StringSliceEqual compares two slices and returns true if they match
func StringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// StringSliceRemove removes from slice 's' any elements which occur on slice 'db'.
func StringSliceRemove(s, db []string) []string {
	var r []string
	for i := 0; i < len(s); i++ {
		found := false
		for _, e := range db {
			if s[i] == e {
				found = true
				break
			}
		}
		if !found {
			r = append(r, s[i])
		}
	}
	return r
}

// StringSliceRemoveMatch removes from slice 's' any elements for which the 'match'
// function returns true.
func StringSliceRemoveMatch(s []string, match func(string) bool) []string {
	var r []string
	for i := 0; i < len(s); i++ {
		if !match(s[i]) {
			r = append(r, s[i])
		}
	}
	return r
}

// uniquify a string slice (i.e., remove duplicate elements)
func StringSliceUniquify(s []string) []string {
	keys := make(map[string]bool)
	result := []string{}
	for _, str := range s {
		if _, ok := keys[str]; !ok {
			keys[str] = true
			result = append(result, str)
		}
	}
	return result
}

// Compares the given mount slices and returns true if the match
func MountSliceEqual(a, b []specs.Mount) bool {
	if len(a) != len(b) {
		return false
	}
	for i, m := range a {
		if m.Destination != b[i].Destination ||
			m.Source != b[i].Source ||
			m.Type != b[i].Type ||
			!StringSliceEqual(m.Options, b[i].Options) {
			return false
		}
	}
	return true
}

// MountSliceRemove removes from slice 's' any elements which occur on slice 'db'; the
// given function is used to compare elements.
func MountSliceRemove(s, db []specs.Mount, cmp func(m1, m2 specs.Mount) bool) []specs.Mount {
	var r []specs.Mount
	for i := 0; i < len(s); i++ {
		found := false
		for _, e := range db {
			if cmp(s[i], e) {
				found = true
				break
			}
		}
		if !found {
			r = append(r, s[i])
		}
	}
	return r
}

// MountSliceRemoveMatch removes from slice 's' any elements for which the 'match'
// function returns true.
func MountSliceRemoveMatch(s []specs.Mount, match func(specs.Mount) bool) []specs.Mount {
	var r []specs.Mount
	for i := 0; i < len(s); i++ {
		if !match(s[i]) {
			r = append(r, s[i])
		}
	}
	return r
}
