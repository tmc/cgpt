//go:build js

package interactive

// NewSession selects the BubbleSession implementation for JS/WASM builds.
func NewSession(cfg Config) (Session, error) {
	// For JS/WASM environments, always use BubbleSession (using the WASM fork)
	return NewBubbleSession(cfg)
}
