package config

import (
	"fmt"
	"reflect"
	"testing"
)

func TestPortRangeValidate(t *testing.T) {
	invalidPortNum := PortRange{From: 0, To: 65536}
	validPortNum := PortRange{From: 19000, To: 20000}

	badParamsTests := []PortRange{
		// [From] is invalid
		{From: invalidPortNum.From, To: validPortNum.To},

		// [To] is invalid
		{From: validPortNum.From, To: invalidPortNum.To},

		// [From] and [To] are invalid
		{From: invalidPortNum.From, To: invalidPortNum.To},

		// each value are valid but [From] is greater than [To]
		{From: validPortNum.To, To: validPortNum.From},
	}

	t.Run("bad port range", func(t *testing.T) {
		for _, test := range badParamsTests {
			if err := test.Validate(); err == nil {
				t.Errorf("PortRange.Validate() didn't work properly when %+v", test)
			}
		}
	})

	goodParamsTests := []PortRange{
		// [To] is greater than [From]
		{From: validPortNum.From, To: validPortNum.To},

		// [From] and [To] are same port
		{From: validPortNum.From, To: validPortNum.From},
	}

	t.Run("good port range", func(t *testing.T) {
		for _, test := range goodParamsTests {
			if err := test.Validate(); err != nil {
				t.Errorf("PortRange.Validate() didn't work properly when %+v", test)
			}
		}
	})
}

func TestConfigValidate(t *testing.T) {
	invalidConfig := Config{
		ControlPort:       0,
		SenderPortRange:   PortRange{From: 0, To: 65536},
		ReceiverPortRange: PortRange{From: 0, To: 65536},
		Count:             0,
		Timeout:           0,
		IP:                IPProtocol{Version: 42, Fallback: true},
	}

	valid := reflect.ValueOf(defaultConfig)

	for i := 0; i < valid.NumField(); i++ {
		fieldName := valid.Type().Field(i).Name
		testTitle := fmt.Sprintf("bad %s", fieldName)

		// Config initialization for testing
		// defaultConfig is also a valid Config
		testConfig := defaultConfig

		switch fieldName {
		case "ControlPort":
			testConfig.ControlPort = invalidConfig.ControlPort
		case "SenderPortRange":
			testConfig.SenderPortRange = invalidConfig.SenderPortRange
		case "ReceiverPortRange":
			testConfig.ReceiverPortRange = invalidConfig.ReceiverPortRange
		case "Count":
			testConfig.Count = invalidConfig.Count
		case "Timeout":
			testConfig.Timeout = invalidConfig.Timeout
		case "IP":
			testConfig.IP = invalidConfig.IP
		}

		t.Run(testTitle, func(t *testing.T) {
			if err := testConfig.Validate(); err == nil {
				t.Errorf("Config.Validate() didn't work properly when %+v", testConfig)
			}
		})
	}
}
