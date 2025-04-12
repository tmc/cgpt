# Testing Terminal UX Features in CGPT

This guide explains how to test the terminal user experience (UX) features in CGPT, including bracketed paste mode, interrupt handling, and slow responses.

## Automated Tests

CGPT includes automated tests for terminal UX features:

```bash
# Run all terminal UX tests
go test -v ./cmd/cgpt/scripttest -run=TestTerminalUX

# Run specific tests
go test -v ./cmd/cgpt/scripttest -run=TestTerminalUXWithBracketedPaste
go test -v ./cmd/cgpt/scripttest -run=TestTerminalUXWithInterrupts
go test -v ./cmd/cgpt/scripttest -run=TestTerminalUXFlags
```

## Manual Testing Scripts

For more interactive testing, use the provided scripts:

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

## Testing Bracketed Paste Mode

Bracketed paste mode optimizes the handling of large paste operations:

1. Run CGPT in interactive mode:
   ```bash
   ./cgpt -c
   ```

2. Copy a large block of text (e.g., 50+ lines)

3. Paste it into the terminal

4. Observe:
   - The paste size indication (e.g., `[Pasted 1.2 KB]`)
   - Reduced flickering during paste
   - Proper handling of the pasted content

## Testing Interrupt Handling

To test interrupt handling:

1. Run CGPT with slow responses:
   ```bash
   ./cgpt --backend=dummy --model=dummy-model --slow-responses -c
   ```

2. Enter some text and press Enter

3. While the response is streaming, press Ctrl+C

4. Observe:
   - The `[Interrupted]` message
   - Return to the prompt

5. At an empty prompt, press Ctrl+C

6. Observe:
   - The "Press Ctrl+C again to exit" message

7. Press Ctrl+C again quickly (within 250ms)

8. Observe:
   - The "Received rapid double interrupt, exiting immediately" message
   - Immediate exit

## Testing Slow Responses

To test slow response handling:

1. Run CGPT with the slow-responses flag:
   ```bash
   ./cgpt --backend=dummy --model=dummy-model --slow-responses -c
   ```

2. Enter some text and press Enter

3. Observe:
   - The slower token-by-token response generation
   - UI responsiveness during generation

## Testing HTTP Record/Replay

To test HTTP recording and replaying:

1. Record mode:
   ```bash
   ./cgpt --backend=dummy --model=dummy-model --http-record=test.httprr -c
   ```

2. Enter some prompts and observe responses

3. Exit CGPT

4. Replay mode:
   ```bash
   ./cgpt --backend=dummy --model=dummy-model --http-record=test.httprr -c
   ```

5. Enter the same prompts and verify the responses match the recorded ones

## Troubleshooting

If you encounter issues with terminal UX features:

1. Check terminal compatibility:
   - Most modern terminals support bracketed paste mode
   - Some older terminals may not support it

2. Verify terminal settings:
   - Some terminal configurations might interfere with special escape sequences

3. Debug mode:
   ```bash
   ./cgpt --debug -c
   ```
   This will show additional information about terminal events
