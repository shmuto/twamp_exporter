package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shmuto/twamp_exporter/config"
	"github.com/shmuto/twamp_exporter/prober"
)

func main() {

	var configFileFlag = flag.String("config.file", "config.yaml", "path to config file")
	var webListeningAddressFlag = flag.String("web.listen-address", "0.0.0.0:9861", "listening addres and port")
	flag.Parse()

	configModules, err := config.LoadConfigFromFile(*configFileFlag)
	if err != nil {
		log.Print("failed to load configuration")
		log.Print(err)
		os.Exit(1)
	}

	log.Print("Listening on " + *webListeningAddressFlag)

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {

		module := r.URL.Query().Get("module")
		if _, ok := configModules[module]; !ok {
			handleError("Module ["+module+"] is not defined.", nil, w, r)
			return
		}

		target := r.URL.Query().Get("target")
		if target == "" {
			handleError("target is not provided.", nil, w, r)
			return
		}

		p := prober.TwampProber{
			ProberConfig: configModules[module],
		}

		registry := p.Test(target)
		if err != nil {
			log.Print(err)
			return
		}

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	http.Handle("/metrics", promhttp.Handler())

	http.ListenAndServe(*webListeningAddressFlag, nil)
}

func handleError(message string, err error, w http.ResponseWriter, r *http.Request) {
	if err != nil {
		log.Print(message, " Reason: ", err.Error())
	} else {
		log.Print(message)
	}
}
