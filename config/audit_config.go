package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type HookConfig struct {
	Hook struct {
		ContextPath string `yaml:"context-path"`
		Audit       struct {
			InformTime []string `yaml:"inform-time"`
			InformCron string   `yaml:"inform-cron"`
		} `yaml:"audit"`
		Apps []string `yaml:"apps"`
	} `yaml:"hook"`
}

func LoadHookConfig(path string) (*HookConfig, error) {
	hookConfig := &HookConfig{}
	hookConfig.Hook.ContextPath = "/hook"
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return hookConfig, err
	}

	if err := yaml.Unmarshal(data, hookConfig); err != nil {
		return hookConfig, err
	}

	return hookConfig, nil
}
