package cgpt

import (
	"time"

	"github.com/tmc/spinner"
)

func spin() func() {
	s := spinner.New(
		spinner.WithFrames(spinner.Dots8),
		spinner.WithIntervalFunc(
			spinner.SpeedupInterval(90*time.Millisecond, 40*time.Millisecond, time.Second*5),
		),
		spinner.WithColorFunc(spinner.GreyPulse(15*time.Millisecond)),
	)
	s.Start()
	return s.Stop
}
