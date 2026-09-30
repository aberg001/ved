package main

import "strings"

// Change is one differing region: removed old lines, added new lines,
// starting at oldStart/newStart (0-based).
type Change struct {
	OldStart, OldEnd int // [OldStart, OldEnd) indexes into old
	NewStart, NewEnd int // [NewStart, NewEnd) indexes into new
}

// DiffLine computes line-level changes between old and new.
// Uses LCS dynamic programming; falls back to coarse whole-buffer diff
// for very large buffers to keep it fast.
func DiffLine(old, new []string) []Change {
	const maxCells = 4_000_000
	if len(old)*len(new) > maxCells {
		// coarse: find common prefix/suffix, everything else is one change
		p := 0
		for p < len(old) && p < len(new) && old[p] == new[p] {
			p++
		}
		s := 0
		for s < len(old)-p && s < len(new)-p && old[len(old)-1-s] == new[len(new)-1-s] {
			s++
		}
		return []Change{{p, len(old) - s, p, len(new) - s}}
	}
	n, m := len(old), len(new)
	// LCS table
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if old[i] == new[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var changes []Change
	i, j := 0, 0
	for i < n && j < m {
		if old[i] == new[j] {
			i++
			j++
			continue
		}
		os, ns := i, j
		for i < n && j < m && old[i] != new[j] {
			if dp[i+1][j] >= dp[i][j+1] {
				i++
			} else {
				j++
			}
		}
		for i < n && j < m && old[i] == new[j] {
			break
		}
		changes = append(changes, Change{os, i, ns, j})
	}
	if i < n || j < m {
		changes = append(changes, Change{i, n, j, m})
	}
	return changes
}

// OldLines / NewLines helpers for a change
func (c Change) OldLines(old []string) []string { return old[c.OldStart:c.OldEnd] }
func (c Change) NewLines(new []string) []string { return new[c.NewStart:c.NewEnd] }

// HasContent reports whether the string is non-blank.
func HasContent(s string) bool { return strings.TrimSpace(s) != "" }
