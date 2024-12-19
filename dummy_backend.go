package cgpt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type DummyBackend struct {
	GenerateText func() string
}

func NewDummyBackend() (*DummyBackend, error) {
	return &DummyBackend{
		GenerateText: func() string { return dummyDefaultText },
	}, nil
}

var dummyDefaultText = "This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend. The quick brown fox jumps over the lazy dog. This pangram contains every letter of the English alphabet at least once. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. This concludes the dummy backend response. Thank you for using the dummy backend! \n"

var dummyVimrcText = "This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend. Here's a basic vimrc configuration:\n\n" +
	"```vimrc\n" +
	`" Basic vim settings
set nocompatible
syntax enable
filetype plugin indent on

" Install vim-plug if not found
if empty(glob('~/.vim/autoload/plug.vim'))
  silent !curl -fLo ~/.vim/autoload/plug.vim --create-dirs
    \ https://raw.githubusercontent.com/junegunn/vim-plug/master/plug.vim
endif

call plug#begin('~/.vim/plugged')
" Tpope essentials
Plug 'tpope/vim-sensible'
Plug 'tpope/vim-surround'
Plug 'tpope/vim-commentary'
Plug 'tpope/vim-fugitive'

" Vim-go
Plug 'fatih/vim-go'

" Theme
Plug 'nanotech/jellybeans.vim'
call plug#end()

" Theme settings
colorscheme jellybeans` + "\n```\n\n" +
	"This concludes the dummy backend response. Thank you for using the dummy backend!  \n"

var dummyPythonText = "This is a dummy backend response. It will stream out a few hundred tokens to simulate a real backend. Here's a Python function to calculate Fibonacci numbers:\n\n" +
	"```python\n" +
	`def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)` + "\n```\n\n" +
	"This concludes the dummy backend response. Thank you for using the dummy backend! \n"

func (d *DummyBackend) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	response, err := d.GenerateContent(ctx, messages, options...)
	if err != nil {
		return "", err
	}
	if len(response.Choices) > 0 {
		return response.Choices[0].Content, nil
	}
	return "", nil
}

func (d *DummyBackend) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	// Determine which response to use based on system prompt and input
	var dummyText string
	hasVimrcPrompt := false
	hasPythonPrompt := false

	for _, msg := range messages {
		if msg.Role == llms.ChatMessageTypeSystem {
			var parts []string
			for _, part := range msg.Parts {
				parts = append(parts, fmt.Sprint(part))
			}
			content := strings.Join(parts, " ")
			if strings.Contains(content, "vimrc expert") {
				hasVimrcPrompt = true
			}
		}
		if msg.Role == llms.ChatMessageTypeHuman {
			var parts []string
			for _, part := range msg.Parts {
				parts = append(parts, fmt.Sprint(part))
			}
			content := strings.Join(parts, " ")
			if strings.Contains(content, "fibonacci") || strings.Contains(content, "Python") {
				hasPythonPrompt = true
			}
		}
	}

	if hasVimrcPrompt {
		dummyText = dummyVimrcText
	} else if hasPythonPrompt {
		dummyText = dummyPythonText
	} else {
		dummyText = dummyDefaultText
	}

	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: dummyText,
			},
		},
	}

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	if opts.StreamingFunc != nil {
		// Stream character by character to preserve formatting
		var buffer strings.Builder
		for i := 0; i < len(dummyText); i++ {
			select {
			case <-ctx.Done():
				return response, ctx.Err()
			default:
				char := string(dummyText[i])
				buffer.WriteString(char)

				if err := opts.StreamingFunc(ctx, []byte(char)); err != nil {
					return response, err
				}

				// Check if we've hit the stop sequence after streaming the character
				if opts.StopSequence != "" && strings.HasSuffix(buffer.String(), opts.StopSequence) {
					// Update response content to include everything up to and including stop sequence
					response.Choices[0].Content = buffer.String()
					return response, nil
				}

				time.Sleep(10 * time.Millisecond) // Reduced delay for character-by-character streaming
			}
		}
		return response, nil
	}

	// Handle non-streaming case
	if opts.StopSequence != "" {
		if idx := strings.Index(dummyText, opts.StopSequence); idx >= 0 {
			response.Choices[0].Content = dummyText[:idx+len(opts.StopSequence)]
		}
	}

	return response, nil
}

func (d *DummyBackend) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	// Dummy embedding (just return a fixed-size vector of zeros)
	return make([]float64, 128), nil
}
