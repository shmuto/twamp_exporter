package config

import (
	"testing"
)

func TestPortRangeValidate(t *testing.T) {
	invalidPortNum := PortRange{From: -1, To: 65536}
	validPortNum := PortRange{From: 19000, To: 20000}

	badParamsTests := []PortRange{
		// [From] is invalid
		{From: invalidPortNum.From, To: validPortNum.To},

		// [To] is invalid
		{From: validPortNum.From, To: invalidPortNum.To},

		// [From] and [To] are invalid
		{From: invalidPortNum.From, To: invalidPortNum.To},

		// [From] is greater than [To]
		{From: invalidPortNum.To, To: invalidPortNum.From},
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
				t.Errorf("PortRange.Validate() didn't work perpery when %+v", test)
			}
		}
	})
}
