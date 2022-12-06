package prober

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shmuto/twamp"
	"github.com/shmuto/twamp_exporter/config"
)

type TwampProber struct {
	ProberConfig config.Config
}

var (
	twampSuccessGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "twamp_success",
		Help: "TWAMP sucess or not",
	})

	twampDurationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "twamp_duration_seconds",
			Help: "measurement result of TWAMP",
		},
		[]string{"direction", "type"},
	)
)

func InitMetrics() *prometheus.Registry {

	registry := prometheus.NewRegistry()
	registry.MustRegister(twampSuccessGauge, twampDurationGauge)

	twampDurationGauge.WithLabelValues("both", "min").Set(0)
	twampDurationGauge.WithLabelValues("both", "avg").Set(0)
	twampDurationGauge.WithLabelValues("both", "max").Set(0)
	twampDurationGauge.WithLabelValues("back", "min").Set(0)
	twampDurationGauge.WithLabelValues("back", "avg").Set(0)
	twampDurationGauge.WithLabelValues("back", "max").Set(0)
	twampDurationGauge.WithLabelValues("forward", "min").Set(0)
	twampDurationGauge.WithLabelValues("forward", "avg").Set(0)
	twampDurationGauge.WithLabelValues("forward", "max").Set(0)
	twampSuccessGauge.Set(0)

	return registry
}

func (prober *TwampProber) Test(hostname string) *prometheus.Registry {

	registry := InitMetrics()

	twampServerIP, err := ResolveHostname(hostname, prober.ProberConfig.IP.Version)
	if err != nil && !prober.ProberConfig.IP.Fallback {
		log.Println(err)
		return registry
	}

	var fallbackIP net.IP = nil
	if prober.ProberConfig.IP.Fallback {
		if prober.ProberConfig.IP.Version == 4 {
			fallbackIP, _ = ResolveHostname(hostname, 6)
		} else {
			fallbackIP, _ = ResolveHostname(hostname, 4)
		}
	}

	if twampServerIP == nil {
		if fallbackIP == nil {
			log.Printf("failed to resolve %s", hostname)
			return registry
		} else {
			twampServerIP = fallbackIP
		}
	}

	var twampServerAddr = net.TCPAddr{IP: twampServerIP, Port: prober.ProberConfig.ControlPort}

	c := twamp.NewClient()
	connection, err := c.Connect(twampServerAddr.String())
	if err != nil {
		if fallbackIP == nil {
			log.Printf("Failed to connect %s. Reason: %s", twampServerAddr.String(), err)
			return registry
		} else {
			twampServerAddr.IP = fallbackIP
			connection, err = c.Connect(twampServerAddr.String())
			if err != nil {
				log.Printf("Failed to connect %s. Reason: %s", fallbackIP.String(), err)
				return registry
			}
		}
	}

	var ipVersion int
	if twampServerAddr.IP.To4() == nil {
		ipVersion = 6
	} else {
		ipVersion = 4
	}

	session, err := connection.CreateSession(
		twamp.TwampSessionConfig{
			ReceiverPort: GetRandomPortFromRange(prober.ProberConfig.ReceiverPortRange),
			SenderPort:   GetRandomPortFromRange(prober.ProberConfig.SenderPortRange),
			Timeout:      int(prober.ProberConfig.Timeout),
			Padding:      int(prober.ProberConfig.Count),
			TOS:          0,
			IPVersion:    ipVersion,
		},
	)
	if err != nil {
		log.Print("Failed to create session. Reason: ", err)
		return registry
	}

	test, err := session.CreateTest()
	if err != nil {
		log.Print("Failed to create test. Reason: ", err)
		return registry
	}

	var twampDurationFwdMin float64 = math.MaxFloat64
	var twampDurationFwdMax float64 = math.MinInt
	var twampDurationBckMin float64 = math.MaxFloat64
	var twampDurationBckMax float64 = math.MinInt
	var twampDurationFwdTotal float64 = 0
	var twampdurationBckTotal float64 = 0

	results := test.RunX(int(prober.ProberConfig.Count), func(result *twamp.TwampResults) {

		twampDurationFwd := result.ReceiveTimestamp.Sub(result.SenderTimestamp).Seconds()
		twampDurationBck := result.FinishedTimestamp.Sub(result.Timestamp).Seconds()

		twampDurationFwdTotal += twampDurationFwd
		twampdurationBckTotal += twampDurationBck
		if twampDurationFwdMin > twampDurationFwd {
			twampDurationFwdMin = twampDurationFwd
		}
		if twampDurationFwdMax < twampDurationFwd {
			twampDurationFwdMax = twampDurationFwd
		}
		if twampDurationBckMin > twampDurationBck {
			twampDurationBckMin = twampDurationBck
		}
		if twampDurationBckMax < twampDurationBck {
			twampDurationBckMax = twampDurationBck
		}
	})

	session.Stop()
	connection.Close()

	twampDurationGauge.WithLabelValues("both", "min").Set(float64(results.Stat.Min.Seconds()))
	twampDurationGauge.WithLabelValues("both", "avg").Set(float64(results.Stat.Avg.Seconds()))
	twampDurationGauge.WithLabelValues("both", "max").Set(float64(results.Stat.Max.Seconds()))

	twampDurationGauge.WithLabelValues("back", "min").Set(float64(twampDurationBckMin))
	twampDurationGauge.WithLabelValues("back", "avg").Set(float64(twampdurationBckTotal / float64(prober.ProberConfig.Count)))
	twampDurationGauge.WithLabelValues("back", "max").Set(float64(twampDurationBckMax))

	twampDurationGauge.WithLabelValues("forward", "min").Set(float64(twampDurationFwdMin))
	twampDurationGauge.WithLabelValues("forward", "avg").Set(float64(twampDurationFwdTotal / float64(prober.ProberConfig.Count)))
	twampDurationGauge.WithLabelValues("forward", "max").Set(float64(twampDurationFwdMax))

	twampSuccessGauge.Set(1)

	return registry
}

func ResolveHostname(hostname string, ipv int) (net.IP, error) {
	if targetIP := net.ParseIP(hostname); targetIP == nil {

		// "ip4" or "ip6". default is "ip6"
		preferredIPVersion := fmt.Sprintf("ip%d", ipv)

		resolver := net.Resolver{}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		targetIPs, err := resolver.LookupIP(ctx, preferredIPVersion, hostname)
		if err != nil {
			return nil, err
		}
		return targetIPs[0], nil
	} else {
		return targetIP, nil
	}

}

func GetRandomPortFromRange(portRange config.PortRange) int {
	if portRange.From == portRange.To {
		return portRange.From
	}
	return portRange.From + rand.Int()%(portRange.To-portRange.From)
}
