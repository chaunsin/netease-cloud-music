package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	confPath *string
	home     string
)

func init() {
	var err error
	home, err = os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	confPath = pflag.String("f", "./config.yaml", "main -f ./.ncmctl/config.yaml")
}

type Config struct {
	v       *viper.Viper
	Version string      `json:"version" yaml:"version"`
	Log     *log.Config `json:"log" yaml:"log"`
	Network *api.Config `json:"network" yaml:"network"`
}

func (c *Config) Validate() error {
	return nil
}

func New() *Config {
	var (
		conf Config
		opts = func(m *mapstructure.DecoderConfig) {
			m.TagName = "yaml"
		}
	)

	v := viper.New()
	v.SetTypeByDefaultValue(true)
	v.SetEnvPrefix("ncmctl")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.AllowEmptyEnv(true)
	v.SetConfigType("yaml")
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./.ncmctl")
	v.AddConfigPath(filepath.Join(home, ".ncmctl"))
	v.AddConfigPath(filepath.Dir(*confPath))
	// v.SetConfigFile(*confPath)
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}
	if err := v.UnmarshalExact(&conf, opts); err != nil {
		panic(err)
	}
	if err := conf.Validate(); err != nil {
		panic(err)
	}
	return &conf
}
