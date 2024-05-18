package cgpt

import (
	"os"
	"path/filepath"

	"github.com/chzyer/readline"
)

var defaultReadlineConfig = &readline.Config{
	Prompt:  "> ",
	VimMode: true,
}

var historyFileName = ".cgpt_history"

func newReadline() (*readline.Instance, error) {
	home, _ := os.UserHomeDir()
	defaultReadlineConfig.HistoryFile = filepath.Join(home, historyFileName)
	return readline.NewEx(defaultReadlineConfig)
}
