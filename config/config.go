package config

import (
	"errors"
	"time"

	"github.com/chaunsin/netease-cloud-music/cookie"
)

type Network struct {
	Debug   bool                       `json:"debug" yaml:"debug"`
	Timeout time.Duration              `json:"timeout" yaml:"timeout"`
	Retry   int                        `json:"retry" yaml:"retry"`
	Cookie  cookie.PersistentJarConfig `json:"cookie" yaml:"cookie"`
}

type UserAgent struct {
	Android []string `json:"android"`
	IOS     []string `json:"ios"`
	Mac     []string `json:"mac"`
	Windows []string `json:"windows"`
	Linux   []string `json:"linux"`
}

type Config struct {
	Network   Network   `json:"network" yaml:"network"`
	UserAgent UserAgent `json:"userAgent" yaml:"userAgent"`
}

func (c *Config) Valid() error {
	if c.Network.Retry < 0 {
		return errors.New("retry is < 0")
	}
	if c.Network.Timeout < 0 {
		return errors.New("timeout is < 0")
	}
	return nil
}
