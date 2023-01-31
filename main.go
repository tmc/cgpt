// Command cgpt is a command line tool for interacting with the OpenAI completion apis.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
)

var flagInput = flag.String("input", "-", "The input text to complete. If '-', read from stdin.")

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	flag.Parse()
	input := *flagInput
	if input == "-" {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		input = string(b)
	}

	r, err := post("", Payload{
		Prompt: input,
	})
	if err != nil {
		return err
	}
	fmt.Println(r.Choices[0].Text)
	return nil
}
