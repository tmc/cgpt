package backends

import (
	// Import all backend packages for registration
	_ "github.com/tmc/cgpt/backends/anthropic"
	_ "github.com/tmc/cgpt/backends/dummy"
	_ "github.com/tmc/cgpt/backends/googleai"
	_ "github.com/tmc/cgpt/backends/ollama"
	_ "github.com/tmc/cgpt/backends/openai"
)
