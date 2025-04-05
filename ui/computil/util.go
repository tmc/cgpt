// Package computil provides utility functions for completion logic,
// inspired by bubbline/computil.
package computil

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FindWord searches backwards and forwards from the cursor position (line, col)
// within the current line's runes (v[line]) to find the boundaries of the "word"
// under the cursor, where words are separated by whitespace.
//
// Returns:
//   - word: The string representation of the word found.
//   - wordStart: The starting column index of the word in the current line's runes.
//   - wordEnd: The ending column index (exclusive) of the word in the current line's runes.
func FindWord(v [][]rune, line, col int) (word string, wordStart, wordEnd int) {
	// Basic bounds check
	if line < 0 || line >= len(v) {
		return "", 0, 0
	}
	curLine := v[line]
	curLen := len(curLine)
	// Clamp column to be within the current line's bounds
	col = clamp(col, 0, curLen)

	// Find beginning of word (scan backwards)
	wordStart = col
	for wordStart > 0 && !unicode.IsSpace(curLine[wordStart-1]) {
		wordStart--
	}

	// Find end of word (scan forwards)
	wordEnd = col
	for wordEnd < curLen && !unicode.IsSpace(curLine[wordEnd]) {
		wordEnd++
	}

	// Extract the word
	if wordStart < wordEnd { // Ensure start is actually before end
		word = string(curLine[wordStart:wordEnd])
	} else {
		word = "" // No word found (e.g., cursor is on whitespace)
		// Adjust start/end to be the cursor position if on whitespace
		wordStart = col
		wordEnd = col
	}

	return word, wordStart, wordEnd
}

// FindLongestCommonPrefix finds the longest common prefix of two strings.
// caseSensitive determines if the comparison ignores case.
func FindLongestCommonPrefix(s1, s2 string, caseSensitive bool) string {
	maxLen := min(len(s1), len(s2))
	for i := 0; i < maxLen; {
		r1, size1 := utf8.DecodeRuneInString(s1[i:])
		r2, size2 := utf8.DecodeRuneInString(s2[i:])

		// Check for mismatch
		if r1 != r2 {
			// If case-insensitive, check uppercase versions
			if !caseSensitive && unicode.ToUpper(r1) == unicode.ToUpper(r2) {
				// Match, continue
			} else {
				return s1[:i] // Mismatch found, return prefix up to this point
			}
		}

		// Ensure both runes had the same size for safety, though this
		// should generally be true if they match or match case-insensitively.
		if size1 != size2 {
			// This case is rare but possible with invalid UTF-8 mixups.
			return s1[:i]
		}
		i += size1 // Advance by the size of the rune
	}
	// If loop completes, the shorter string is the prefix (or they are equal)
	return s1[:maxLen]
}

// FindLongestCommonPrefixSlice finds the longest common prefix among a slice of strings.
func FindLongestCommonPrefixSlice(strs []string, caseSensitive bool) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	// Start with the first string as the potential prefix
	prefix := strs[0]

	// Compare with the rest
	for i := 1; i < len(strs); i++ {
		prefix = FindLongestCommonPrefix(prefix, strs[i], caseSensitive)
		if prefix == "" {
			// If at any point the prefix becomes empty, there's no common prefix
			return ""
		}
	}
	return prefix
}

// Flatten converts the 2D rune representation of the editor's value
// and the cursor's 2D position (line, col) into a single string and
// a corresponding 1D byte offset.
func Flatten(v [][]rune, line, col int) (string, int) {
	var buf strings.Builder
	byteOffset := -1 // Initialize to -1 to indicate cursor position not found yet

	// Basic bounds check for line index
	if line < 0 {
		line = 0
	}
	if line >= len(v) {
		line = len(v) - 1
	}
	if line < 0 {
		line = 0
	} // Handle case where v might be empty

	for rowIdx, row := range v {
		// Add newline character before appending the next row (except for the first row)
		if rowIdx > 0 {
			buf.WriteByte('\n')
		}

		// If this is the line the cursor is on
		if line == rowIdx {
			// Clamp column to be within the current row's bounds
			col = clamp(col, 0, len(row))
			// Write the part of the line before the cursor
			buf.WriteString(string(row[:col]))
			// Record the current length of the buffer as the cursor's byte offset
			byteOffset = buf.Len()
			// Write the part of the line after the cursor
			buf.WriteString(string(row[col:]))
		} else {
			// If this is not the cursor line, write the whole line
			buf.WriteString(string(row))
		}
	}

	// If the cursor was at the very end of the input, the offset would still be -1.
	// Set it to the total length of the string in this case.
	if byteOffset == -1 {
		byteOffset = buf.Len()
	}

	return buf.String(), byteOffset
}

// Helpers (copied from Bubbline's textarea)
func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
