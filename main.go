// Command cgpt is a command line tool for interacting with the OpenAI completion APIs.
//
// The -continuous flag will run the completion API in a loop, using the previous output as the
// input for the next request. It will run after two newlines are entered.
//
// The -json flag will output the full JSON response from the API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

var (
	flagInput      = flag.String("input", "-", "The input text to complete. If '-', read from stdin.")
	flagConfig     = flag.String("config", "config.yaml", "Path to the configuration file")
	flagContinuous = flag.Bool("continuous", false, "Run in continuous mode")
	flagJSON       = flag.Bool("json", false, "Output JSON")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg, err := loadConfig(*flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Missing API key in config. Please set the OPENAI_API_KEY environment variable or add it to the config file.")
		os.Exit(1)
	}
	if *flagContinuous {
		err = runContinuousCompletion(ctx, cfg)
	} else {
		err = runOneShotCompletion(ctx, cfg)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
