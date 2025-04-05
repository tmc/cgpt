package completion

import (
	"fmt"
	"io"
)

// This function has been moved to completion.go

// OutputCompatibilityError outputs a compatibility error message
func OutputCompatibilityError(stderr io.Writer) {
	fmt.Fprintln(stderr, "Error: This version of cgpt requires updated packages. Please run `go get -u github.com/tmc/cgpt/...` to update.")
}
