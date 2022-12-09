package prober

import (
	"testing"

	"github.com/shmuto/twamp_exporter/config"
)

func TestGetRandomPort(t *testing.T) {
	minPortRange := config.PortRange{From: 1, To: 1}
	maxPortRange := config.PortRange{From: 1, To: 65535}

	t.Run("minimum port range", func(t *testing.T) {
		port := GetRandomPortFromRange(minPortRange)
		if port != 1 {
			t.Errorf("GetRandomPortRange() didn't work properly when %+v", minPortRange)
		}
	})

	t.Run("maximum port range", func(t *testing.T) {
		port := GetRandomPortFromRange(maxPortRange)
		if 65535 < port || port < 1 {
			t.Errorf("GetRandomPortRange() didn't work properly when %+v", maxPortRange)
		}
	})
}
