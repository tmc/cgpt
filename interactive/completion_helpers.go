// This file provides helpers similar to bubbline/editline/complete.go
package interactive

import (
	// "sort" // Removed unused import
	// rw "github.com/mattn/go-runewidth" // Removed unused import
	"github.com/tmc/cgpt/ui/completion" // Use local completion package
	// "github.com/tmc/cgpt/ui/computil"   // Removed unused import
)

// --- Implementation of completion.Entry for simple strings ---
type simpleEntry struct {
	title string
	desc  string
}

func (e simpleEntry) Title() string       { return e.title }
func (e simpleEntry) Description() string { return e.desc }

// --- Implementation of completion.Values for simple strings ---
type simpleValues struct {
	category string
	entries  []completion.Entry
}

func (sv simpleValues) NumCategories() int              { return 1 }
func (sv simpleValues) CategoryTitle(catIdx int) string { return sv.category }
func (sv simpleValues) NumEntries(catIdx int) int       { return len(sv.entries) }
func (sv simpleValues) Entry(catIdx, entryIdx int) completion.Entry {
	if catIdx == 0 && entryIdx >= 0 && entryIdx < len(sv.entries) {
		return sv.entries[entryIdx]
	}
	return nil // Or handle error appropriately
}

// --- Implementation of Candidate for simple strings ---
// This defines how a selected completion replaces text in the editor.
type simpleCandidate struct {
	replacement string
	deleteLeft  int // How many runes left of the cursor to delete
	moveRight   int // How many runes right of the cursor to delete (part of the original word)
}

func (sc simpleCandidate) Replacement() string { return sc.replacement }
func (sc simpleCandidate) DeleteLeft() int     { return sc.deleteLeft }
func (sc simpleCandidate) MoveRight() int      { return sc.moveRight }

// --- Implementation of Completions for simple strings ---
// This combines Values and Candidate logic for simple cases.
type simpleCompletions struct {
	simpleValues
	deleteLeft  int
	moveRight   int
	origWordLen int // Store original word length if needed for Candidate logic
}

func (sc *simpleCompletions) Candidate(e completion.Entry) Candidate {
	return simpleCandidate{
		replacement: e.Title(),
		deleteLeft:  sc.deleteLeft,
		moveRight:   sc.moveRight,
	}
}

// --- Helpers for creating Completions --- (Removed functions using undefined types)

// --- Prefill Logic (Commented Out - Requires significant rework) ---

/*
// wordsCompletion implements the Completions interface for simple string lists.
type wordsCompletion struct {
	vals        completion.Values
	moveRight   int // How many runes to move right *within the original word*
	deleteLeft  int // How many runes to delete left *from the original word start*
	origWordLen int // Original length of the word being replaced
}

// Embed methods from completion.Values
func (s *wordsCompletion) NumCategories() int              { return s.vals.NumCategories() }
func (s *wordsCompletion) CategoryTitle(catIdx int) string { return s.vals.CategoryTitle(catIdx) }
func (s *wordsCompletion) NumEntries(catIdx int) int       { return s.vals.NumEntries(catIdx) }
func (s *wordsCompletion) Entry(catIdx, entryIdx int) completion.Entry {
	return s.vals.Entry(catIdx, entryIdx)
}

// Candidate converts the simple string Entry into a Candidate.
func (s *wordsCompletion) Candidate(e completion.Entry) Candidate {
	return wordsEntryCandidate{
		repl:        e.Title(),
		moveRight:   s.moveRight,   // Keep original moveRight relative to cursor
		deleteTotal: s.origWordLen, // Delete the entire original word length
	}
}

// wordsEntryCandidate implements the Candidate interface for simple strings.
type wordsEntryCandidate struct {
	repl        string
	moveRight   int // How far right cursor was within original word
	deleteTotal int // How many total runes to delete (entire original word)
}

func (c wordsEntryCandidate) Replacement() string { return c.repl }

// MoveRight should now be relative to the *end* of the replacement string.
// Since we replace the whole word, the cursor ends up after the replacement.
func (c wordsEntryCandidate) MoveRight() int { return 0 } // Cursor ends after replacement
// DeleteLeft should delete the entire original word.
func (c wordsEntryCandidate) DeleteLeft() int { return c.deleteTotal }


// computePrefill computes the longest common prefix and returns adjusted completions.
func computePrefill(comp Completions) (
	hasPrefill bool,
	moveRight, deleteLeft int, // Params relative to the *original* word being completed
	prefill string,
	newCompletions Completions, // Adjusted completions starting after the prefill
) {
	if comp == nil || comp.NumCategories() == 0 || (comp.NumCategories() == 1 && comp.NumEntries(0) == 0) {
		return false, 0, 0, "", nil // No completions
	}

	var candidates []string
	initialDeleteLeft := -1
	initialMoveRight := -1
	numCandidates := 0

	// Iterate through all entries to check consistency and gather replacements
	numCats := comp.NumCategories()
	for catIdx := 0; catIdx < numCats; catIdx++ {
		numE := comp.NumEntries(catIdx)
		for eIdx := 0; eIdx < numE; eIdx++ {
			e := comp.Entry(catIdx, eIdx)
			c := comp.Candidate(e)
			// Use the *original* word boundaries from the completion generator
			cdl := c.DeleteLeft() // This now represents the length of the original word part left of cursor
			cmr := c.MoveRight()  // This now represents the length of the original word part right of cursor

			if initialDeleteLeft == -1 { // First candidate sets the expectation
				initialDeleteLeft = cdl
				initialMoveRight = cmr
			} else if cdl != initialDeleteLeft || cmr != initialMoveRight {
				// Inconsistent delete/move parameters among candidates
				return false, 0, 0, "", comp
			}
			candidates = append(candidates, c.Replacement())
			numCandidates++
		}
	}

	if numCandidates == 0 {
		return false, 0, 0, "", nil
	}

	// Calculate total original word length using the consistent params
	originalWordLen := initialDeleteLeft + initialMoveRight

	if numCandidates == 1 {
		// If only one candidate, the prefill *is* the full candidate
		return true, initialMoveRight, originalWordLen, candidates[0], nil
	}

	// Find longest common prefix among candidates
	sort.Strings(candidates)                                                                        // Ensure sorting for correct prefix finding
	prefix := computil.FindLongestCommonPrefix(candidates[0], candidates[len(candidates)-1], false) // Assume case-sensitive

	// Calculate the part of the prefix that extends beyond the original word's length
	// This happens if the common prefix is longer than the word being completed.
	prefixToAdd := ""
	if len(prefix) > initialDeleteLeft {
		prefixToAdd = prefix[initialDeleteLeft:]
	}

	if prefixToAdd == "" {
		// No actual characters to add beyond what was typed, or no common prefix found
		// Return original completions but update delete/move based on *current* cursor pos
		// This scenario might mean just showing the list without prefilling.
		// We need the current cursor position relative to the start of the word (initialDeleteLeft)
		// to correctly calculate how much to delete/move.
		return false, initialMoveRight, originalWordLen, "", comp
	}

	// Create adjusted completions (shift candidates)
	adjustedComp := &shiftedCompletions{
		orig: comp,
		// Shift amount is based on the *width* of the part we are adding
		shift:       rw.StringWidth(prefixToAdd),
		origWordLen: originalWordLen,
	}

	// hasPrefill = true
	// moveRight = how many original chars were right of cursor (initialMoveRight)
	// deleteLeft = how many total original chars to delete (originalWordLen)
	// prefill = the part of the common prefix *to add* (prefixToAdd)
	return true, initialMoveRight, originalWordLen, prefixToAdd, adjustedComp
}

// shiftedCompletions wraps original completions to adjust candidate parameters.
type shiftedCompletions struct {
	orig        Completions
	shift       int // Width of the prefilled part that was added
	origWordLen int // Length of the original word being replaced
}

// Embed methods from original Completions
func (s *shiftedCompletions) NumCategories() int              { return s.orig.NumCategories() }
func (s *shiftedCompletions) CategoryTitle(catIdx int) string { return s.orig.CategoryTitle(catIdx) }
func (s *shiftedCompletions) NumEntries(catIdx int) int       { return s.orig.NumEntries(catIdx) }
func (s *shiftedCompletions) Entry(catIdx, entryIdx int) completion.Entry {
	return s.orig.Entry(catIdx, entryIdx)
}

// Candidate adjusts the DeleteLeft parameter.
func (s *shiftedCompletions) Candidate(e completion.Entry) Candidate {
	origCandidate := s.orig.Candidate(e)
	return shiftedCandidate{
		orig:        origCandidate,
		shift:       s.shift,
		origWordLen: s.origWordLen,
	}
}

// shiftedCandidate wraps an original Candidate with adjusted parameters.
type shiftedCandidate struct {
	orig        Candidate
	shift       int // Width of the added prefill
	origWordLen int // Length of original word
}

func (sc shiftedCandidate) Replacement() string { return sc.orig.Replacement() }
func (sc shiftedCandidate) MoveRight() int      { return 0 } // Cursor always ends after replacement
// DeleteLeft should now delete the *entire original word* plus the *prefill part* we added.
// However, the editor applies the prefill *first*, then calls Update with the
// completion selection. At that point, DeleteLeft should delete the part of the
// original word *before* the cursor + the prefill we added.
// The `Candidate` interface needs rethinking for prefill. Let's stick to the
// simpler model for now: prefill inserts text, then completion replaces the *original* word.
func (sc shiftedCandidate) DeleteLeft() int { return sc.origWordLen }
*/
