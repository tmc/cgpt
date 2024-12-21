package main

import (
	"io"
	"os"
	"testing"
)

func TestContinuousModeWithPipe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEOF  bool
		wantCont bool
	}{
		{
			name:     "simple pipe input",
			input:    "test input\n",
			wantEOF:  true,
			wantCont: true,
		},
		{
			name:     "empty pipe input",
			input:    "",
			wantEOF:  true,
			wantCont: true,
		},
		{
			name:     "multiline pipe input",
			input:    "line 1\nline 2\nline 3\n",
			wantEOF:  true,
			wantCont: true,
		},
		{
			name:     "non continuous mode with flags",
			input:    "test input\n",
			wantEOF:  true,
			wantCont: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock stdin
			r, w := io.Pipe()
			oldStdin := os.Stdin
			os.Stdin = r
			defer func() { os.Stdin = oldStdin }()

			// Setup test args
			args := []string{"cgpt"}
			if !tt.wantCont {
				args = append(args, "--no-continuous")
			}

			// Setup test
			opts, _, err := initFlags(args, r)
			if err != nil {
				t.Fatal(err)
			}

			// Verify continuous mode is enabled/disabled as expected
			if got := opts.Continuous; got != tt.wantCont {
				t.Errorf("continuous = %v, want %v", got, tt.wantCont)
			}

			// Write test input
			go func() {
				if tt.input != "" {
					w.Write([]byte(tt.input))
				}
				if tt.wantEOF {
					w.Close()
				}
			}()

			// Verify graceful exit by attempting to read after EOF
			buf := make([]byte, 1)
			n, err := r.Read(buf)
			if n != 0 || err != io.EOF {
				t.Errorf("expected EOF error after graceful exit, got n=%d err=%v", n, err)
			}
		})
	}
}


func TestEOFHandlingInContinuousMode(t *testing.T) {
	mock, err := newMockIO()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Test that EOF properly terminates continuous mode
	input := "test input\n"
	done := make(chan struct{})

	go func() {
		defer close(done)
		mock.stdin.Write([]byte(input))
		mock.stdin.Close() // Send EOF
	}()

	// Verify behavior
	opts, _, err := initFlags([]string{"cgpt"}, mock.stdin)
	if err != nil {
		t.Fatal(err)
	}

	// Verify continuous mode is enabled for pipe input
	if !opts.Continuous {
		t.Error("continuous mode not enabled for pipe input")
	}

	// Wait for EOF to be sent
	<-done

	// Verify graceful exit by attempting to read after EOF
	buf := make([]byte, 1)
	n, err := mock.stdin.Read(buf)
	if n != 0 || err != io.EOF {
		t.Errorf("expected EOF error after graceful exit, got n=%d err=%v", n, err)
	}
}
