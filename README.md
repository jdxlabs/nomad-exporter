# nomad-exporter

A fork of [Nomon/nomad-exporter](https://github.com/Nomon/nomad-exporter) which
exposes metrics not exposed by the default Nomad Prometheus
[telemetry endpoint](https://www.nomadproject.io/docs/agent/telemetry.html).

## Local Development

```
go run nomad-exporter.go --nomad.server $NOMAD_ADDR
curl -s localhost:9172/metrics | grep -E '^nomad_'
```

## Docker

```
docker build -t nomad-exporter:latest .
```

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| nomad_up | Was the last query of Nomad successful. | |
| nomad_exporter_raft_peers | How many peers (servers) are in the Raft cluster. | |
| nomad_exporter_nodes | the number of nodes | datacenter,class,drain,status |
| nomad_exporter_jobs | the number of jobs | status,type |
| nomad_exporter_allocs | the number of allocs | client_status,desired_status,job_type,node_id |
| nomad_exporter_evals | the number of evaluations | status |
| nomad_exporter_tasks | the number of tasks | state,failed,job_type,node_id |
| nomad_exporter_deployments | the number of deployments | status,job_id |
| nomad_exporter_deployment_task_group_desired_canaries | the number of desired canaries for the task group | job_id,deployment_id,task_group,promoted |
| nomad_exporter_deployment_task_group_desired_total | the number of desired allocs for the task group | job_id,deployment_id,task_group,promoted |
| nomad_exporter_deployment_task_group_placed_allocs | the number of placed allocs for the task group | job_id,deployment_id,task_group,promoted |
| nomad_exporter_deployment_task_group_healthy_allocs | the number of healthy allocs for the task group | job_id,deployment_id,task_group,promoted |
| nomad_exporter_deployment_task_group_unhealthy_allocs | the number of unhealthy allocs for the task group | job_id,deployment_id,task_group,promoted |
