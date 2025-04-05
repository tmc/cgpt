//go:build !js

package options

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func _setupViper(v *viper.Viper, flagSet *pflag.FlagSet) {
}
