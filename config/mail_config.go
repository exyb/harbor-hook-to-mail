package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type MailConfig struct {
	Email struct {
		Type   string `yaml:"type"`
		Server string `yaml:"server"`
		Port   int    `yaml:"port"`
		Sender struct {
			Address  string `yaml:"address"`
			Password string `yaml:"password"`
		} `yaml:"sender"`
		Receiver []string `yaml:"receiver"`
		CC       []string `yaml:"cc"`
		Body     struct {
			Type    string `yaml:"type"`
			Subject string `yaml:"subject"`
			Message string `yaml:"message"`
		} `yaml:"body"`
		Attachments []string `yaml:"attachments"`
	} `yaml:"email"`
}



func LoadEmailConfig(path string) (*MailConfig, error) {
	config := &MailConfig{}
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
