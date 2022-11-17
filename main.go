package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/tcaine/twamp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ControlPort  int `yaml:"controlPort"`
	SenderPort   int `yaml:"senderPort"`
	ReceiverPort int `yaml:"receiverPort"`
	Count        int `yaml:"count"`
	Timeout      int `yaml:"timeout"`
}

var (
	DefaultConfig = Config{
		ControlPort:  862,
		SenderPort:   19000,
		ReceiverPort: 19000,
		Count:        100,
		Timeout:      1,
	}
)

func main() {

	log.Print("loading configuration file")
	// load config
	configFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Print("no configuration file found")
	}

	config := Config{}

	if configFile == nil {
		log.Print("loading default configuration")
		config = DefaultConfig
	} else {
		log.Print("loading configuration")
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
			log.Print("failed to load configuration")
		os.Exit(1)
	}
	}

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {

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

		twampServerIPandPort := targetIP.String() + ":" + strconv.Itoa(config.ControlPort)

		// connect twamp server
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

		results := test.RunX(config.Count, func(result *twamp.TwampResults) {
		})

		session.Stop()
		connection.Close()

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
