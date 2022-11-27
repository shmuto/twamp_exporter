# twamp_exporter

config.yaml
```yaml
controlPort: 862
senderPort: 19000
receiverPort: 19000
count: 100
timeout: 1
```

```
go run main.go -web.listen-address=0.0.0.0:8080 -config.file=config.yaml
```

```
$ curl http://localhost:8080/probe?target=203.0.113.1

# HELP twamp_duration_seconds measurement result of TWAMP
# TYPE twamp_duration_seconds gauge
twamp_duration_seconds{direction="back",type="avg"} -2.6173188784
twamp_duration_seconds{direction="back",type="max"} -2.147483648
twamp_duration_seconds{direction="back",type="min"} -2.637886237
twamp_duration_seconds{direction="both",type="avg"} 0.001345781
twamp_duration_seconds{direction="both",type="max"} 0.001393601
twamp_duration_seconds{direction="both",type="min"} 0.001330202
twamp_duration_seconds{direction="forward",type="avg"} 2.6186586471
twamp_duration_seconds{direction="forward",type="max"} 2.639215443
twamp_duration_seconds{direction="forward",type="min"} 2.147483647
# HELP twamp_success TWAMP sucess or not
# TYPE twamp_success gauge
twamp_success 1
```
