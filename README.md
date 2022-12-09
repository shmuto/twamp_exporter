# twamp_exporter

config.yaml
```yaml
twamp:
  controlPort: 862
  senderPortRange: 
    from: 19000
    to: 20000
  receiverPortRange: 
    from: 19000
    to: 20000
  count: 10
  timeout: 1
  ip:
    fallback: true
    version: 6
```

```
go run main.go -web.listen-address=0.0.0.0:8080 -config.file=config.yaml
```

```
$ curl http://localhost:8080/probe?target=example.com&module=twamp

# HELP twamp_duration_seconds measurement results of TWAMP
# TYPE twamp_duration_seconds gauge
twamp_duration_seconds{direction="back",type="avg"} -0.7642895023000001
twamp_duration_seconds{direction="back",type="max"} -0.738620447
twamp_duration_seconds{direction="back",type="min"} -0.789350204
twamp_duration_seconds{direction="both",type="avg"} 0.001725047
twamp_duration_seconds{direction="both",type="max"} 0.001984096
twamp_duration_seconds{direction="both",type="min"} 0.001406498
twamp_duration_seconds{direction="forward",type="avg"} 0.7659862023999999
twamp_duration_seconds{direction="forward",type="max"} 0.790752407
twamp_duration_seconds{direction="forward",type="min"} 0.740574478
# HELP twamp_ip_protocol IP protocol version used in TWAMP test
# TYPE twamp_ip_protocol gauge
twamp_ip_protocol 4
# HELP twamp_server_info TWAMP server information
# TYPE twamp_server_info gauge
twamp_server_info{address="203.0.113.1",hostname="example.com"} 0
# HELP twamp_success TWAMP sucess or not
# TYPE twamp_success gauge
twamp_success 1
```


# Usage Example

## docker-compose.yml

```yml
version: '3.9'
services:
  twamp:
    image: ghcr.io/shmuto/twamp_exporter:latest
    network_mode: host
```

## prometheus.yml

```yml
scrape_configs:
  - job_name: 'twamp'
    static_configs:
    - targets:
      - 203.0.113.1
      - example.com
    params:
      module: ['twamp']
    metrics_path: /probe
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: localhost:9861
```
