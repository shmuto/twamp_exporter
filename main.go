package main

import (
	"context"
	"flag"
	"math"
	"net"
	"net/http"
	"os"
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
	configFile, err := os.ReadFile(*configFileFlag)
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

		module := r.URL.Query().Get("module")
		if _, ok := configModules[module]; !ok {
			handleError("Module ["+module+"] is not defined.", nil, registry, w, r)
			return
		}

		target := r.URL.Query().Get("target")

		resolver := net.Resolver{}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var preferredIPVersion string
		if configModules[module].IP.Version == 4 {
			preferredIPVersion = "ip4"
		} else {
			preferredIPVersion = "ip6"
		}

		var targetIP net.IP

		if targetIP = net.ParseIP(target); targetIP == nil {

			if !configModules[module].IP.Fallback {

				targetIPs, err := resolver.LookupIP(ctx, preferredIPVersion, target)
				if err != nil {
					handleError("Failed to resolve ["+target+"].", err, registry, w, r)
					return
				}
				targetIP = targetIPs[0]

			} else {

				targetIPs, err := resolver.LookupIP(ctx, "ip", target)
				if err != nil {
					handleError("Failed to resolve ["+target+"].", err, registry, w, r)
					return
				}

				var fallbackIP *net.IP
				for _, ip := range targetIPs {
					if preferredIPVersion == "ip4" {
						if ip.To4() == nil {
							fallbackIP = &ip
						} else {
							targetIP = ip
							break
						}
					} else if preferredIPVersion == "ip6" {
						if ip.To4() == nil {
							targetIP = ip
							break
						} else {
							fallbackIP = &ip
						}
					}
				}
				if targetIP == nil {
					targetIP = *fallbackIP
				}
			}
		}

		var ipVersion = 6
		if targetIP.To4() != nil {
			ipVersion = 4
		}

		twampServerAddr := net.TCPAddr{IP: targetIP, Port: configModules[module].ControlPort}

		// TWAMP process
		c := twamp.NewClient()
		connection, err := c.Connect(twampServerAddr.String())
		if err != nil {
			handleError("Connection failed to "+twampServerAddr.String(), err, registry, w, r)
			return
		}

		session, err := connection.CreateSession(
			twamp.TwampSessionConfig{
				ReceiverPort: config.GetRandomPortFromRange(configModules[module].ReceiverPortRange),
				SenderPort:   config.GetRandomPortFromRange(configModules[module].SenderPortRange),
				Timeout:      int(configModules[module].Timeout),
				Padding:      int(configModules[module].Count),
				TOS:          0,
				IPVersion:    ipVersion,
			},
		)
		if err != nil {
			log.Print("Failed to create session. Reason: ", err)
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		test, err := session.CreateTest()
		if err != nil {
			log.Print("Failed to create test. Reason: ", err)
			twampSuccessGauge.Set(0)
			h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
			h.ServeHTTP(w, r)
			return
		}

		var twampDurationFwdMin float64 = math.MaxFloat64
		var twampDurationFwdMax float64 = math.MinInt
		var twampDurationBckMin float64 = math.MaxFloat64
		var twampDurationBckMax float64 = math.MinInt
		var twampDurationFwdTotal float64 = 0
		var twampdurationBckTotal float64 = 0

		results := test.RunX(int(configModules[module].Count), func(result *twamp.TwampResults) {

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
		twampDurationGauge.WithLabelValues("back", "avg").Set(float64(twampdurationBckTotal / float64(configModules[module].Count)))
		twampDurationGauge.WithLabelValues("back", "max").Set(float64(twampDurationBckMax))

		twampDurationGauge.WithLabelValues("forward", "min").Set(float64(twampDurationFwdMin))
		twampDurationGauge.WithLabelValues("forward", "avg").Set(float64(twampDurationFwdTotal / float64(configModules[module].Count)))
		twampDurationGauge.WithLabelValues("forward", "max").Set(float64(twampDurationFwdMax))

		twampSuccessGauge.Set(1)

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.ListenAndServe(*webListeningAddressFlag, nil)
}

func handleError(message string, err error, registry *prometheus.Registry, w http.ResponseWriter, r *http.Request) {
	if err != nil {
		log.Print(message, " Reason: ", err.Error())
	} else {
		log.Print(message)
	}
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
