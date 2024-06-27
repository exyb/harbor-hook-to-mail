package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type RegistryConfig struct {
	Registry struct {
		Address string `yaml:"address"`
		Auth    struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"auth"`
	} `yaml:"registry"`
}

func LoadRegistryConfig(path string) (*RegistryConfig, error) {
	config := &RegistryConfig{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
