package completion

import (
	"io"
	"os"
	"time"

	"github.com/tmc/spinner"
	"golang.org/x/term"
)

func spin(pos int, out io.Writer) func() {
	s := spinner.New(
		spinner.WithFrames(spinner.Dots8),
		spinner.WithWriter(out),
		spinner.WithIntervalFunc(
			spinner.SpeedupInterval(90*time.Millisecond, 40*time.Millisecond, time.Second*5),
		),
		spinner.WithColorFunc(spinner.GreyPulse(15*time.Millisecond)),
		spinner.WithPosition(pos),
	)
	s.Start()
	return s.Stop
}

func isInputFromTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
