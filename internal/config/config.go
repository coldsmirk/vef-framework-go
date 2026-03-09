package config

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"github.com/coldsmirk/vef-framework-go/config"
	ilog "github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/mapx"
)

var decodeUsingConfigTagOption viper.DecoderConfigOption = func(c *mapstructure.DecoderConfig) {
	c.TagName = "config"
	c.IgnoreUntaggedFields = true
	c.DecodeHook = mapx.DecoderHook
}

type ViperConfig struct {
	v *viper.Viper
}

func (v *ViperConfig) Unmarshal(key string, target any) error {
	return v.v.UnmarshalKey(key, target, decodeUsingConfigTagOption)
}

func newConfig() (config.Config, error) {
	v := viper.NewWithOptions(
		viper.EnvKeyReplacer(strings.NewReplacer(".", "_")),
		viper.KeyDelimiter("."),
		viper.WithLogger(ilog.NewSLogger("config", 3, log.LevelWarn)),
	)
	v.SetEnvPrefix(config.EnvKeyPrefix)
	v.AllowEmptyEnv(true)
	v.AutomaticEnv()

	v.SetConfigName("application")
	v.SetConfigType("toml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("$VEF_CONFIG_PATH")
	v.AddConfigPath(".")
	v.AddConfigPath("../configs")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return &ViperConfig{v: v}, nil
}
