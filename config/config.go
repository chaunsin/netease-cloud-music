package config

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/database"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var HomeDir string

var (
	//go:embed config.yaml
	defaultConfigByte []byte
	defaultConfig     *Config
)

func init() {
	var err error
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal(defaultConfigByte, &defaultConfig); err != nil {
		panic(fmt.Sprintf("defaultConfig.Unmarshal: %s", err))
	}
	// defaultConfig.ReplaceMagicVariables("HOME", HomeDir)
	if err := defaultConfig.Validate(); err != nil {
		panic(fmt.Sprintf("defaultConfig.Validate: %s", err))
	}
}

type Config struct {
	v        *viper.Viper
	Version  string           `json:"version" yaml:"version"`
	Log      *log.Config      `json:"log" yaml:"log"`
	Network  *api.Config      `json:"network" yaml:"network"`
	Database *database.Config `json:"database" yaml:"database"`
}

func (c *Config) Validate() error {
	return nil
}

func GetDefault() *Config {
	return defaultConfig
}

func New(cfgPath ...string) (*Config, error) {
	var (
		conf Config
		opts = viper.DecodeHook(func(m *mapstructure.DecoderConfig) {
			m.TagName = "yaml"
		})
		_cfgPath string
	)
	if len(cfgPath) > 0 {
		_cfgPath = cfgPath[0]
	}

	v := viper.New()
	v.SetTypeByDefaultValue(true)
	v.SetEnvPrefix("ncmctl")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.AllowEmptyEnv(true)
	v.SetConfigType("yaml")
	v.SetConfigFile(_cfgPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("ReadInConfig: %w", err)
	}
	if err := v.UnmarshalExact(&conf, opts); err != nil {
		return nil, fmt.Errorf("UnmarshalExact: %w", err)
	}
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	return &conf, nil
}

// ReplaceMagicVariables 替换配置文件中的魔法变量。注意该方法只能调用一次再次调用则不会生效.
func (c *Config) ReplaceMagicVariables(name, value string) (*Config, bool) {

	var (
		isset   bool
		mapping = func(k string) string {
			switch k {
			case name:
				isset = true
				return value
			}
			return ""
		}
	)

	c.Log.Rotate.Filename = os.Expand(c.Log.Rotate.Filename, mapping)
	c.Network.Cookie.Filepath = os.Expand(c.Network.Cookie.Filepath, mapping)
	c.Database.Path = os.Expand(c.Database.Path, mapping)
	return c, isset
}
