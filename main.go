package main

import (
	"flag"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/shmuto/twamp"
	"github.com/shmuto/twamp_exporter/config"
	"gopkg.in/yaml.v2"
)

func main() {

	var configFileFlag = flag.String("config.file", "config.yaml", "path to config file")
	var webListeningAddressFlag = flag.String("web.listen-address", "localhost:2112", "listening addres and port")
	flag.Parse()

	// load config
	configFile, err := ioutil.ReadFile(*configFileFlag)
	if err != nil || configFile == nil {
		log.Print("failed to load " + *configFileFlag)
		os.Exit(1)
	}

	configModules := map[string]config.Config{}
	log.Print("loading configuration from " + *configFileFlag)
	err = yaml.Unmarshal(configFile, &configModules)
	if err != nil {
		log.Print("failed to load configuration")
		os.Exit(1)
	}

	log.Print("Listening on " + *webListeningAddressFlag)

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {

		registry := prometheus.NewRegistry()
		twampSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_success",
			Help: "TWAMP sucess or not",
		})

		twampDurationGauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "twamp_duration_seconds",
				Help: "measurement result of TWAMP",
			},
			[]string{"direction", "type"},
		)

		registry.MustRegister(twampDurationGauge)
		registry.MustRegister(twampSuccessGauge)

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

		targetIP := net.ParseIP(r.URL.Query().Get("target"))

		if targetIP == nil {
			log.Print("target is not provided")
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		module := r.URL.Query().Get("module")
		if _, ok := configModules[module]; !ok {
			log.Print("module [" + module + "] is not defined")
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		twampServerIPandPort := targetIP.String() + ":" + strconv.Itoa(int(configModules[module].ControlPort))

		// TWAMP process
		c := twamp.NewClient()
		connection, err := c.Connect(twampServerIPandPort)
		if err != nil {
			log.Print("Connection failed to " + twampServerIPandPort)
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		session, err := connection.CreateSession(
			twamp.TwampSessionConfig{
				ReceiverPort: config.GetRandomPortFromRange(configModules[module].ReceiverPortRange),
				SenderPort:   config.GetRandomPortFromRange(configModules[module].SenderPortRange),
				Timeout:      int(configModules[module].Timeout),
				Padding:      int(configModules[module].Count),
				TOS:          0,
			},
		)
		if err != nil {
			log.Print("failed to create session")
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		test, err := session.CreateTest()
		if err != nil {
			log.Print("failed to create test")
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		var twampDurationFwdMin time.Duration = time.Duration(math.MaxInt32)
		var twampDurationFwdMax time.Duration = time.Duration(math.MinInt32)
		var twampDurationBckMin time.Duration = time.Duration(math.MaxInt32)
		var twampDurationBckMax time.Duration = time.Duration(math.MinInt32)
		var twampDurationFwdTotal float64 = 0
		var twampdurationBckTotal float64 = 0

		results := test.RunX(int(configModules[module].Count), func(result *twamp.TwampResults) {

			twampDurationFwd := result.ReceiveTimestamp.Sub(result.SenderTimestamp)
			twampDurationBck := result.FinishedTimestamp.Sub(result.Timestamp)

			twampDurationFwdTotal += twampDurationFwd.Seconds()
			twampdurationBckTotal += twampDurationBck.Seconds()
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

		twampDurationGauge.WithLabelValues("back", "min").Set(float64(twampDurationBckMin.Seconds()))
		twampDurationGauge.WithLabelValues("back", "avg").Set(float64(twampdurationBckTotal / float64(configModules[module].Count)))
		twampDurationGauge.WithLabelValues("back", "max").Set(float64(twampDurationBckMax.Seconds()))

		twampDurationGauge.WithLabelValues("forward", "min").Set(float64(twampDurationFwdMin.Seconds()))
		twampDurationGauge.WithLabelValues("forward", "avg").Set(float64(twampDurationFwdTotal / float64(configModules[module].Count)))
		twampDurationGauge.WithLabelValues("forward", "max").Set(float64(twampDurationFwdMax.Seconds()))

		twampSuccessGauge.Set(1)

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.ListenAndServe(*webListeningAddressFlag, nil)
}
