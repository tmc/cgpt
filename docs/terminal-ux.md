# Terminal UX Features in CGPT

This document describes the terminal user experience (UX) features implemented in CGPT, focusing on optimizations for input handling, paste operations, and interrupt handling.

## Bracketed Paste Mode

Bracketed paste mode is a terminal feature that sends special markers when content is pasted into the terminal. CGPT utilizes this feature to optimize performance during paste operations.

### How It Works

1. When enabled, the terminal sends `ESC[200~` before pasted content and `ESC[201~` after the pasted content.
2. CGPT detects these markers and temporarily disables screen redraws during the paste operation.
3. This significantly reduces flickering and improves performance for large paste operations.

### Implementation Details

- Bracketed paste mode is automatically enabled at startup with: `\x1b[?2004h`
- Disabled on exit with: `\x1b[?2004l`
- Paste detection implemented in `handleBracketedPaste()` function in `interactive/readline.go`
- Paste size reporting shows the amount of pasted content (e.g., `[Pasted 1.2 KB]`)

## Interrupt Handling

CGPT provides clear feedback when operations are interrupted:

1. **Input clearing**: When Ctrl+C is pressed during text entry, the message `Input cleared` is shown
2. **Processing interruption**: When Ctrl+C is pressed during response generation, `[Interrupted]` is shown
3. **Exiting**: When Ctrl+C is pressed at an empty prompt, `Exiting...` is shown

### Double-Interrupt Feature

A "double-interrupt" feature is implemented to allow users to force exit by pressing Ctrl+C twice in rapid succession:

- First Ctrl+C at empty prompt shows "Press Ctrl+C again to exit"
- Second Ctrl+C within a short time window (250ms) forces immediate exit

## Slow Response Testing

The `--slow-responses` flag simulates slower response generation:

- Normal mode: ~40ms per word
- Slow mode: ~300ms per word

This is useful for testing the UI responsiveness and interrupt handling during long-running operations.

## HTTP Record/Replay

The `--http-record` flag enables recording and replaying of HTTP interactions:

- Record mode: `--http-record=file.httprr` records all HTTP interactions to the specified file
- Replay mode: Uses the same flag to replay recorded interactions instead of making real HTTP requests

This feature is particularly useful for:
- Creating deterministic tests
- Testing without network connectivity
- Reducing API usage during development

## Testing Terminal UX Features

To test these features, use the provided scripts:

```bash
# Run all terminal UX tests
./scripts/terminal-ux-test.sh

# Test specific features
./scripts/terminal-ux-test.sh paste    # Test bracketed paste mode
./scripts/terminal-ux-test.sh slow     # Test slow responses
./scripts/terminal-ux-test.sh interrupt # Test interrupt handling (manual)

# Record HTTP interactions for tests
./scripts/terminal-ux-test.sh --record
```

## Recommended Terminal Settings

For the best experience with large pastes in CGPT:

1. Use a terminal that supports bracketed paste mode (most modern terminals do)
2. Consider increasing terminal scrollback buffer for large operations
3. For very large pastes (multiple MB), consider using file input methods instead
