package config

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
	return nil
}
