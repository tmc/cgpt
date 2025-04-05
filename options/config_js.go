//go:build js

package options

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	document js.Value

	lsEnv map[string]any
)

func init() {
	// Override Getenv for JS environment to potentially use localStorage
	Getenv = func(key string) string {
		if lsEnv != nil {
			if value, ok := lsEnv[key]; ok {
				return fmt.Sprintf("%v", value)
			}
		}
		// Fallback or default behavior if needed
		return ""
	}
}

func _setupViper(v *viper.Viper, flagSet *pflag.FlagSet) {
	fmt.Println("cgpt: setupViper called (js)") // Add (js) for clarity
	lsConfig := js.Global().Get("localStorage").Call("getItem", "cgpt")

	fmt.Println("cgpt: localStorage content:", lsConfig)
	config := map[string]any{}
	lsEnv = make(map[string]any) // Initialize lsEnv

	if !lsConfig.IsNull() && !lsConfig.IsUndefined() && lsConfig.Type() == js.TypeString && lsConfig.String() != "" {
		// Parse the JSON string into a map
		err := json.Unmarshal([]byte(lsConfig.String()), &config)
		if err != nil {
			fmt.Println("cgpt: error unmarshalling localStorage JSON:", err)
			// Decide how to handle error: return? proceed without localStorage?
		} else {
			fmt.Println("cgpt: Parsed localStorage config:", config)
			for key, value := range config {
				fmt.Println("cgpt: Setting from localStorage - key:", key, "value:", value)
				// Set in viper and also store for Getenv override
				v.Set(key, value)
				lsEnv[key] = value
			}
		}
	} else {
		fmt.Println("cgpt: localStorage is null, undefined, not a string, or empty")
	}

	// Read URL params and set them in Viper (overriding localStorage)
	// Ensure window and location exist before accessing search
	window := js.Global().Get("window")
	if window.IsUndefined() {
		fmt.Println("cgpt: window is undefined, cannot read URL params")
		return
	}
	location := window.Get("location")
	if location.IsUndefined() {
		fmt.Println("cgpt: window.location is undefined, cannot read URL params")
		return
	}
	search := location.Get("search")
	if search.IsUndefined() || search.Type() != js.TypeString {
		fmt.Println("cgpt: window.location.search is undefined or not a string")
		return
	}

	urlParams := js.Global().Get("URLSearchParams").New(search)
	fmt.Println("cgpt: url params string:", search.String())

	// Iterate through known viper keys first
	for _, k := range v.AllKeys() {
		if urlParams.Call("has", k).Bool() {
			val := urlParams.Call("get", k)
			if !val.IsNull() && !val.IsUndefined() && val.Type() == js.TypeString {
				fmt.Println("cgpt: Setting from URL param (viper key) -", k, ":", val.String())
				v.Set(k, val.String())
				// Also update lsEnv for Getenv consistency? Or assume viper takes precedence?
				// lsEnv[k] = val.String()
			}
		}
	}

	// Iterate through flags to ensure URL params override defaults/config for flags too
	flagSet.VisitAll(func(f *pflag.Flag) {
		if urlParams.Call("has", f.Name).Bool() {
			v := urlParams.Call("get", f.Name)
			if !v.IsNull() && !v.IsUndefined() && v.Type() == js.TypeString {
				fmt.Println("cgpt: Setting flag from URL param -", f.Name, ":", v.String())
				// Use flagSet.Set to ensure correct type parsing if possible,
				// though URL params are strings. Viper might handle this via v.Set above.
				// Setting via flagSet might be redundant if viper handles it, but safer?
				err := flagSet.Set(f.Name, v.String())
				if err != nil {
					fmt.Printf("cgpt: Error setting flag %s from URL param: %v\n", f.Name, err)
				}
				// Ensure viper also reflects the flag value set from URL param
				v.Set(f.Name, v.String())
			}
		}
	})
	fmt.Println("cgpt: setupViper finished (js)")
}
