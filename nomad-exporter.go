package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "sync"
    //"time"

    "github.com/hashicorp/nomad/api"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/common/version"
)

const (
    namespace = "nomad_exporter"
)

var (
    upDesc = prometheus.NewDesc(
        prometheus.BuildFQName(namespace, "", "up"),
        "Was the last query of Nomad successful.",
        nil, nil,
    )
    clusterServersDesc = prometheus.NewDesc(
        prometheus.BuildFQName(namespace, "", "raft_peers"),
        "How many peers (servers) are in the Raft cluster.",
        nil, nil,
    )

    nodeCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "nodes",
        Help:      "the number of nodes",
    },
        []string{"datacenter", "class", "drain", "status"},
    )

    jobCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "jobs",
        Help:      "the number of jobs",
    },
        []string{"status", "type"},
    )

    allocCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "allocs",
        Help:      "the number of allocs",
    },
        []string{
            "client_status",
            "desired_status",
            "job_type",
            //"job_id",
            //"task_group",
            "node_id",
        },
    )

    evalCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "evals",
        Help:      "the number of evaluations",
    },
        []string{"status"},
    )

    taskCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "tasks",
        Help:      "the number of tasks",
    },
        []string{
            "state",
            "failed",
            "job_type",
            "node_id",
        },
    )

    deploymentCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployments",
        Help:      "the number of deployments",
    },
        []string{
            "status",
            "job_id",
        },
    )

    deploymentTaskGroupDesiredCanaries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployment_task_group_desired_canaries",
        Help:      "the number of desired canaries for the task group",
    },
        []string{
            "job_id",
            "deployment_id",
            "task_group",
            "promoted",
        },
    )
    deploymentTaskGroupDesiredTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployment_task_group_desired_total",
        Help:      "the number of desired allocs for the task group",
    },
        []string{
            "job_id",
            "deployment_id",
            "task_group",
            "promoted",
        },
    )
    deploymentTaskGroupPlacedAllocs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployment_task_group_placed_allocs",
        Help:      "the number of placed allocs for the task group",
    },
        []string{
            "job_id",
            "deployment_id",
            "task_group",
            "promoted",
        },
    )
    deploymentTaskGroupHealthyAllocs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployment_task_group_healthy_allocs",
        Help:      "the number of healthy allocs for the task group",
    },
        []string{
            "job_id",
            "deployment_id",
            "task_group",
            "promoted",
        },
    )
    deploymentTaskGroupUnhealthyAllocs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Namespace: namespace,
        Name:      "deployment_task_group_unhealthy_allocs",
        Help:      "the number of unhealthy allocs for the task group",
    },
        []string{
            "job_id",
            "deployment_id",
            "task_group",
            "promoted",
        },
    )
)

type Exporter struct {
    client *api.Client
}

func NewExporter(cfg *api.Config) (*Exporter, error) {
    client, err := api.NewClient(cfg)
    if err != nil {
        return nil, err
    }
    return &Exporter{
        client: client,
    }, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
    ch <- upDesc
    ch <- clusterServersDesc
}

func collectPeersMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    peers, err := e.client.Status().Peers()
    if err != nil {
        ch <- prometheus.MustNewConstMetric(
            upDesc, prometheus.GaugeValue, 0,
        )
        return err
    }

    ch <- prometheus.MustNewConstMetric(
        upDesc, prometheus.GaugeValue, 1,
    )

    ch <- prometheus.MustNewConstMetric(
        clusterServersDesc, prometheus.GaugeValue, float64(len(peers)),
    )

    return nil
}

func collectNodeMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    nodeCount.Reset()

    nodes, _, err := e.client.Nodes().List(&api.QueryOptions{})
    if err != nil {
        return err
    }

    for _, node := range nodes {
        nodeCount.With(prometheus.Labels{
            "datacenter": node.Datacenter,
            "class":      node.NodeClass,
            "drain":      strconv.FormatBool(node.Drain),
            "status":     node.Status,
        }).Add(1)
    }

    nodeCount.Collect(ch)

    return nil
}

func collectJobMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    jobCount.Reset()

    jobs, _, err := e.client.Jobs().List(&api.QueryOptions{})
    if err != nil {
        return err
    }

    for _, job := range jobs {
        jobCount.With(prometheus.Labels{
            "type":   job.Type,
            "status": job.Status,
        }).Add(1)
    }

    jobCount.Collect(ch)

    return nil
}

func collectMetricsForSingleAlloc(e *Exporter, w *sync.WaitGroup, allocStub *api.AllocationListStub) {
    defer w.Done()

    alloc, _, err := e.client.Allocations().Info(allocStub.ID, &api.QueryOptions{})
    if err != nil {
        logError(err)
        return
    }

    job := alloc.Job

    allocCount.With(prometheus.Labels{
        "client_status":  alloc.ClientStatus,
        "desired_status": alloc.DesiredStatus,
        "job_type":       *job.Type,
        //"job_id":         alloc.JobID,
        //"task_group":     alloc.TaskGroup,
        "node_id": alloc.NodeID,
    }).Add(1)

    taskStates := alloc.TaskStates

    for _, task := range taskStates {
        taskCount.With(prometheus.Labels{
            "state":    task.State,
            "failed":   strconv.FormatBool(task.Failed),
            "job_type": *job.Type,
            "node_id":  alloc.NodeID,
        }).Add(1)
    }
}

func collectAllocMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    allocCount.Reset()
    taskCount.Reset()

    allocStubs, _, err := e.client.Allocations().List(&api.QueryOptions{})
    if err != nil {
        return err
    }

    // get all the allocs in parallel with goroutines.
    // then use the WaitGroup to wait for all of them all to finish.
    var w sync.WaitGroup

    for _, allocStub := range allocStubs {
        w.Add(1)
        go collectMetricsForSingleAlloc(e, &w, allocStub)
    }

    w.Wait()
    allocCount.Collect(ch)
    taskCount.Collect(ch)
    return nil
}

func collectEvalMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    evalCount.Reset()

    evals, _, err := e.client.Evaluations().List(&api.QueryOptions{})
    if err != nil {
        return err
    }

    for _, eval := range evals {
        evalCount.With(prometheus.Labels{
            "status": eval.Status,
        }).Add(1)
    }

    evalCount.Collect(ch)

    return nil
}

func collectDeploymentMetrics(e *Exporter, ch chan<- prometheus.Metric) error {
    deploymentCount.Reset()
    deploymentTaskGroupDesiredCanaries.Reset()
    deploymentTaskGroupDesiredTotal.Reset()
    deploymentTaskGroupPlacedAllocs.Reset()
    deploymentTaskGroupHealthyAllocs.Reset()
    deploymentTaskGroupUnhealthyAllocs.Reset()

    deployments, _, err := e.client.Deployments().List(&api.QueryOptions{})
    if err != nil {
        return err
    }

    for _, dep := range deployments {
        taskGroups := dep.TaskGroups

        deploymentCount.With(prometheus.Labels{
            "status": dep.Status,
            "job_id": dep.JobID,
        }).Add(1)

        for taskGroupName, taskGroup := range taskGroups {
            deploymentTaskGroupDesiredCanaries.WithLabelValues(dep.JobID, dep.ID, taskGroupName, strconv.FormatBool(taskGroup.Promoted)).Set(float64(taskGroup.DesiredCanaries))
            deploymentTaskGroupDesiredTotal.WithLabelValues(dep.JobID, dep.ID, taskGroupName, strconv.FormatBool(taskGroup.Promoted)).Set(float64(taskGroup.DesiredTotal))
            deploymentTaskGroupPlacedAllocs.WithLabelValues(dep.JobID, dep.ID, taskGroupName, strconv.FormatBool(taskGroup.Promoted)).Set(float64(taskGroup.PlacedAllocs))
            deploymentTaskGroupHealthyAllocs.WithLabelValues(dep.JobID, dep.ID, taskGroupName, strconv.FormatBool(taskGroup.Promoted)).Set(float64(taskGroup.HealthyAllocs))
            deploymentTaskGroupUnhealthyAllocs.WithLabelValues(dep.JobID, dep.ID, taskGroupName, strconv.FormatBool(taskGroup.Promoted)).Set(float64(taskGroup.UnhealthyAllocs))
        }
    }

    deploymentCount.Collect(ch)
    deploymentTaskGroupDesiredCanaries.Collect(ch)
    deploymentTaskGroupDesiredTotal.Collect(ch)
    deploymentTaskGroupPlacedAllocs.Collect(ch)
    deploymentTaskGroupHealthyAllocs.Collect(ch)
    deploymentTaskGroupUnhealthyAllocs.Collect(ch)

    return nil
}

// Collect collects nomad metrics
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
    err := collectPeersMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    err = collectNodeMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    err = collectJobMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    err = collectAllocMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    err = collectEvalMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    err = collectDeploymentMetrics(e, ch)
    if err != nil {
        logError(err)
        return
    }

    //var w sync.WaitGroup
    //for _, a := range runningAllocs {
    //w.Add(1)
    //go func(a *api.AllocationListStub) {
    //defer w.Done()
    //alloc, _, err := e.client.Allocations().Info(a.ID, &api.QueryOptions{})
    //if err != nil {
    //logError(err)
    //return
    //}

    //stats, err := e.client.Allocations().Stats(alloc, &api.QueryOptions{})
    //if err != nil {
    //logError(err)
    //return
    //}
    //node, _, err := e.client.Nodes().Info(alloc.NodeID, &api.QueryOptions{})
    //if err != nil {
    //logError(err)
    //return
    //}
    //for taskName, taskStats := range stats.Tasks {
    //ch <- prometheus.MustNewConstMetric(
    //taskCPUPercent, prometheus.GaugeValue, taskStats.ResourceUsage.CpuStats.Percent, alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //ch <- prometheus.MustNewConstMetric(
    //taskCPUTotalTicks, prometheus.GaugeValue, taskStats.ResourceUsage.CpuStats.TotalTicks, alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //ch <- prometheus.MustNewConstMetric(
    //taskMemoryRssBytes, prometheus.GaugeValue, float64(taskStats.ResourceUsage.MemoryStats.RSS), alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //}
    //ch <- prometheus.MustNewConstMetric(
    //allocationCPU, prometheus.GaugeValue, stats.ResourceUsage.CpuStats.Percent, alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //ch <- prometheus.MustNewConstMetric(
    //allocationCPUThrottled, prometheus.GaugeValue, float64(stats.ResourceUsage.CpuStats.ThrottledTime), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //ch <- prometheus.MustNewConstMetric(
    //allocationMemory, prometheus.GaugeValue, float64(stats.ResourceUsage.MemoryStats.RSS), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //ch <- prometheus.MustNewConstMetric(
    //allocationMemoryLimit, prometheus.GaugeValue, float64(alloc.Resources.MemoryMB), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
    //)
    //}(a)
    //}
    //w.Wait()
}

func main() {
    var (
        showVersion   = flag.Bool("version", false, "Print version information.")
        listenAddress = flag.String("web.listen-address", ":9172", "Address to listen on for web interface and telemetry.")
        metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
        nomadServer   = flag.String("nomad.server", "http://localhost:4646", "HTTP API address of a Nomad server or agent.")
    )
    flag.Parse()

    if *showVersion {
        fmt.Fprintln(os.Stdout, version.Print("nomad_exporter"))
        os.Exit(0)
    }
    cfg := api.DefaultConfig()
    cfg.Address = *nomadServer

    exporter, err := NewExporter(cfg)
    if err != nil {
        log.Fatal(err)
    }

    prometheus.MustRegister(exporter)

    http.Handle(*metricsPath, prometheus.Handler())
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`<html>
             <head><title>Nomad Exporter</title></head>
             <body>
             <h1>Nomad Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
    })

    log.Println("Listening on", *listenAddress)
    log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func logError(err error) {
    log.Println("Query error", err)
    return
}
