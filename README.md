# nomad-exporter

A fork of [alexebird/nomad-exporter](https://github.com/alexebird/nomad-exporter) which
exposes less metrics to relieve the Prometheus Server
[telemetry endpoint](https://www.nomadproject.io/docs/agent/telemetry.html).

## Local Development

```
# prerequesites
go get -v github.com/hashicorp/nomad/api
go get -v github.com/prometheus/client_golang/prometheus
go get -v github.com/prometheus/common/version
export NOMAD_ADDR=http://localhost:4646

go run nomad-exporter.go --nomad.server $NOMAD_ADDR
curl -s localhost:9172/metrics | grep -E '^nomad_'
```

## Docker

```
docker build --no-cache -t nomad-exporter .
```

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| nomad_exporter_raft_peers | How many peers (servers) are in the Raft cluster. | |
| nomad_exporter_nodes | the number of nodes | datacenter,class,drain,status |
| nomad_exporter_jobs | the number of jobs | status,type |
| nomad_exporter_allocs | the number of allocs | client_status,desired_status,job_type,node_id |
