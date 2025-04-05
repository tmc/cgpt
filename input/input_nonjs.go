//go:build !js

package input

import "io"

func stdinAvailable(i io.Reader) bool {
	return true
}
