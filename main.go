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
	"gopkg.in/yaml.v2"
)

type Config struct {
	ControlPort  int16 `yaml:"controlPort"`
	SenderPort   int16 `yaml:"senderPort"`
	ReceiverPort int16 `yaml:"receiverPort"`
	Count        int   `yaml:"count"`
	Timeout      int   `yaml:"timeout"`
}

var DefaultConfig = Config{
	ControlPort:  862,
	SenderPort:   19000,
	ReceiverPort: 19000,
	Count:        100,
	Timeout:      1,
}

func main() {

	var configFileFlag = flag.String("config.file", "config.yaml", "path to config file")
	var webListeningAddressFlag = flag.String("web.listen-address", "localhost:2112", "listening addres and port")
	flag.Parse()

	// load config
	configFile, err := ioutil.ReadFile(*configFileFlag)
	if err != nil {
		log.Print("no configuration file found")
	}

	config := Config{}

	if configFile == nil {
		log.Print("loading default configuration")
		config = DefaultConfig
	} else {
		log.Print("loading configuration from " + *configFileFlag)
		err = yaml.Unmarshal(configFile, &config)
		if err != nil {
			log.Print("failed to load configuration")
			os.Exit(1)
		}
	}

	log.Print("Listening on " + *webListeningAddressFlag)

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {

		twampSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "twamp_success",
			Help: "TWAMP sucess or not",
		})

		targetIP := net.ParseIP(r.URL.Query().Get("target"))

		if targetIP == nil {
			log.Print("target is not provided")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(twampSuccessGauge)

		twampServerIPandPort := targetIP.String() + ":" + strconv.Itoa(config.ControlPort)

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
				ReceiverPort: config.ReceiverPort,
				SenderPort:   config.SenderPort,
				Timeout:      config.Timeout,
				Padding:      config.Count,
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

		results := test.RunX(config.Count, func(result *twamp.TwampResults) {
			log.Print(time.Now())
			log.Print(twamp.NewTwampTimestamp(time.Now()))
			log.Print(result.SenderTimestamp)
			log.Print(result.ReceiveTimestamp)

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

		// setupMetrics
		twampDurationGauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "twamp_duration_seconds",
				Help: "measurement result of TWAMP",
			},
			[]string{"direction", "type"},
		)

		registry.MustRegister(twampDurationGauge)

		twampDurationGauge.WithLabelValues("both", "min").Set(float64(results.Stat.Min.Seconds()))
		twampDurationGauge.WithLabelValues("both", "avg").Set(float64(results.Stat.Avg.Seconds()))
		twampDurationGauge.WithLabelValues("both", "max").Set(float64(results.Stat.Max.Seconds()))

		twampDurationGauge.WithLabelValues("back", "min").Set(float64(twampDurationBckMin.Seconds()))
		twampDurationGauge.WithLabelValues("back", "avg").Set(float64(twampdurationBckTotal / float64(config.Count)))
		twampDurationGauge.WithLabelValues("back", "max").Set(float64(twampDurationBckMax.Seconds()))

		twampDurationGauge.WithLabelValues("forward", "min").Set(float64(twampDurationFwdMin.Seconds()))
		twampDurationGauge.WithLabelValues("forward", "avg").Set(float64(twampDurationFwdTotal / float64(config.Count)))
		twampDurationGauge.WithLabelValues("forward", "max").Set(float64(twampDurationFwdMax.Seconds()))

		twampSuccessGauge.Set(1)

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.ListenAndServe(*webListeningAddressFlag, nil)
}
