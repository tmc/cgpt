package cgpt

func spin() func() {
	// spinner := spinner.New(spinner.CharSets[14], 50*time.Millisecond)
	// spinner.Start()
	// return spinner.Stop
	return func() {}
}
