package main

import (
	"io"
	"os"
)

type mockIO struct {
	stdin   *os.File
	stdout  *os.File
	stderr  *os.File
	cleanup func()
}

func newMockIO() (*mockIO, error) {
	// Create pipes for stdin, stdout, and stderr
	stdin_r, stdin_w := io.Pipe()
	stdout_r, stdout_w := io.Pipe()
	stderr_r, stderr_w := io.Pipe()

	// Store original file descriptors
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	// Create mock IO with cleanup function
	mock := &mockIO{
		stdin:  os.NewFile(stdin_r.(*os.File).Fd(), "stdin"),
		stdout: os.NewFile(stdout_w.(*os.File).Fd(), "stdout"),
		stderr: os.NewFile(stderr_w.(*os.File).Fd(), "stderr"),
		cleanup: func() {
			// Restore original file descriptors
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			// Close all pipes
			stdin_r.Close()
			stdin_w.Close()
			stdout_r.Close()
			stdout_w.Close()
			stderr_r.Close()
			stderr_w.Close()
		},
	}

	// Set standard file descriptors to use pipes
	os.Stdin = mock.stdin
	os.Stdout = mock.stdout
	os.Stderr = mock.stderr

	return mock, nil
}

func (m *mockIO) Close() {
	if m.cleanup != nil {
		m.cleanup()
	}
}
