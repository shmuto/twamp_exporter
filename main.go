package main

import (
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/tcaine/twamp"
)

func main() {
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {

		twampMinRTTGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_min_rtt",
			Help: "TWAMP Minimum RTT",
		})
		twampMaxRTTGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_max_rtt",
			Help: "TWAMP Maximum RTT",
		})
		twampAvgRTTGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_avg_rtt",
			Help: "TWAMP Average RTT",
		})
		twampSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_success",
			Help: "TWAMP sucess or not",
		})

		targetIP := net.ParseIP(r.URL.Query().Get("target"))

		if targetIP == nil {
			log.Print("target IP is not provided")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(twampSuccessGauge)

		// connect twamp server
		c := twamp.NewClient()
		connection, err := c.Connect(targetIP.String() + ":862")
		if err != nil {
			log.Print("Connection failed to " + targetIP.String() + ":862")
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		session, err := connection.CreateSession(
			twamp.TwampSessionConfig{
				ReceiverPort: 19000,
				SenderPort:   19000,
				Timeout:      1,
				Padding:      100,
				TOS:          0,
			},
		)
		if err != nil {
			log.Print("could not establishe the session")
			twampSuccessGauge.Set(0)
			return
		}

		test, err := session.CreateTest()
		if err != nil {
			log.Print("coould not create the test")
			twampSuccessGauge.Set(0)
			return
		}

		count := 100
		results := test.RunX(count, func(result *twamp.TwampResults) {
		})

		session.Stop()
		connection.Close()

		registry.MustRegister(twampMinRTTGauge)
		registry.MustRegister(twampMaxRTTGauge)
		registry.MustRegister(twampAvgRTTGauge)

		twampMinRTTGauge.Set(float64(results.Stat.Min.Seconds()))
		twampMaxRTTGauge.Set(float64(results.Stat.Max.Seconds()))
		twampAvgRTTGauge.Set(float64(results.Stat.Avg.Seconds()))
		twampSuccessGauge.Set(1)

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.ListenAndServe(":2112", nil)
}
