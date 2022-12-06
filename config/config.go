package config

import (
	"errors"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ControlPort       int        `yaml:"controlPort"`
	SenderPortRange   PortRange  `yaml:"senderPortRange"`
	ReceiverPortRange PortRange  `yaml:"receiverPortRange"`
	Count             int        `yaml:"count"`
	Timeout           int        `yaml:"timeout"`
	IP                IPProtocol `yaml:"ip"`
}

type IPProtocol struct {
	Version  int  `yaml:"version"`
	Fallback bool `yaml:"fallback"`
}

type PortRange struct {
	From int `yaml:"from"`
	To   int `yaml:"to"`
}

var defaultConfig = Config{
	ControlPort:       862,
	SenderPortRange:   PortRange{From: 19000, To: 20000},
	ReceiverPortRange: PortRange{From: 19000, To: 20000},
	Count:             100,
	Timeout:           1,
	IP:                IPProtocol{Version: 6, Fallback: true},
}

func (s *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*s = defaultConfig
	type plain Config
	if err := unmarshal((*plain)(s)); err != nil {
		return err
	}

	log.Print("checking config...")
	if err := Validate(*s); err != nil {
		return err
	}

	return nil
}

func LoadConfigFromFile(filename string) (map[string]Config, error) {

	// load config
	configFile, err := os.ReadFile(filename)
	if err != nil || configFile == nil {
		return nil, err
	}

	configModules := map[string]Config{}
	log.Printf("loading configuration from %s", filename)
	err = yaml.Unmarshal(configFile, &configModules)
	if err != nil {
		return nil, err
	}

	return configModules, nil
}

func Validate(config Config) error {
	// TWAMP control port range should be in 1 to 65535
	if 65535 < config.ControlPort && config.ControlPort < 1 {
		return errors.New("the value of [ControlPort] must be between 1 and 65535")
	}

	// TWAMP test port validation
	if err := config.ReceiverPortRange.Validate(); err != nil {
		return err
	}
	if err := config.SenderPortRange.Validate(); err != nil {
		return err
	}

	// test count must be positive integer
	if config.Count < 1 {
		return errors.New("the value of [Count] must be positive integer")
	}

	// timeout must be positive integer
	if config.Timeout < 1 {
		return errors.New("the value of [Timeout] must be positive integer")
	}

	// IP.Version must be 4 or 6
	if config.IP.Version != 4 && config.IP.Version != 6 {
		return errors.New("the value of [IP.Version] must be 4 or 6")
	}

	return nil
}

func (p *PortRange) Validate() error {
	// .From and .To must be in 1 to 65535
	if (1 > p.From) || (p.From > 65535) || (1 > p.To) || (p.To > 65535) {
		return errors.New("the port range is out of bounds")
	}

	// PortRange.From must not be greater than PortRange.To
	if p.From > p.To {
		return errors.New("the start of the port range is greater than the end")
	}

	return nil
}
