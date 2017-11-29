package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	//"sync"
	//"time"

	"github.com/hashicorp/nomad/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

const (
	namespace = "nomad"
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of Nomad successful.",
		nil, nil,
	)
	clusterServers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "raft_peers"),
		"How many peers (servers) are in the Raft cluster.",
		nil, nil,
	)
	nodeCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "nodes"),
		"Describe node counts",
		[]string{"datacenter", "class", "drain", "status"}, nil,
	)
	jobCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "jobs"),
		"How many jobs are there in the cluster.",
		[]string{"status"}, nil,
	)
	allocationCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocations"),
		"How many allocations are there in the cluster.",
		[]string{"client_status"}, nil,
	)
	evaluationCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "evaluations"),
		"How many evaluations are there in the cluster.",
		[]string{"status"}, nil,
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

// Describe implements Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- clusterServers
	ch <- nodeCount
	ch <- jobCount
	ch <- allocationCount
	ch <- evaluationCount
}

// Collect collects nomad metrics
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	peers, err := e.client.Status().Peers()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		logError(err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)

	ch <- prometheus.MustNewConstMetric(
		clusterServers, prometheus.GaugeValue, float64(len(peers)),
	)

	nodes, _, err := e.client.Nodes().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}
	//var nodeCounts map[string]int = nodesGroupedByLabels(nodes)
	nodeCountVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name: "node_count",
			Help: "the node count",
		},
		[]string{"datacenter", "class", "drain", "status"},
	)
	for _, node := range nodes {
		nodeCountVec.With(prometheus.Labels{
			"datacenter": node.Datacenter,
			"class": node.NodeClass,
			"drain": strconv.FormatBool(node.Drain),
			"status": node.Status,
		}).Add(1)
	}
	nodeCountVec.Collect(ch)

	jobs, _, err := e.client.Jobs().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}
	var jobCounts map[string]int = jobsByStatus(jobs)
	for status, count := range jobCounts {
		ch <- prometheus.MustNewConstMetric(
			jobCount, prometheus.GaugeValue, float64(count), status,
		)
	}

	allocs, _, err := e.client.Allocations().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}
	var allocCounts map[string]int = allocationsByStatus(allocs)
	for status, count := range allocCounts {
		ch <- prometheus.MustNewConstMetric(
			allocationCount, prometheus.GaugeValue, float64(count), status,
		)
	}

	evals, _, err := e.client.Evaluations().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}
	var evalCounts map[string]int = evaluationsByStatus(evals)
	for status, count := range evalCounts {
		ch <- prometheus.MustNewConstMetric(
			evaluationCount, prometheus.GaugeValue, float64(count), status,
		)
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

//func nodesGroupedByLabels(nodes []*api.NodeListStub) map[string]int {
	//var counts map[string]int = make(map[string]int)
	//var (
		//currDc string
		//currClass string
		//currDrain string
		//currStatus string
	//)

	//counts[strings.ToLower("initializing")] = 0
	//counts[strings.ToLower("ready")] = 0
	//counts[strings.ToLower("down")] = 0

	//for _, node := range nodes {
		//currStatus = strings.ToLower(node.Status)
		//counts[currStatus] += 1
	//}

	//return counts
//}

func evaluationsByStatus(evals []*api.Evaluation) map[string]int {
	var counts map[string]int = make(map[string]int)
	var currStatus string

	counts[strings.ToLower("pending")] = 0
	counts[strings.ToLower("blocked")] = 0
	counts[strings.ToLower("complete")] = 0
	counts[strings.ToLower("failed")] = 0
	counts[strings.ToLower("canceled")] = 0

	for _, eval := range evals {
		currStatus = strings.ToLower(eval.Status)
		counts[currStatus] += 1
	}

	return counts
}

func allocationsByStatus(allocs []*api.AllocationListStub) map[string]int {
	var counts map[string]int = make(map[string]int)
	var currStatus string

	counts[strings.ToLower("pending")] = 0
	counts[strings.ToLower("running")] = 0
	counts[strings.ToLower("complete")] = 0
	counts[strings.ToLower("failed")] = 0
	counts[strings.ToLower("lost")] = 0

	for _, alloc := range allocs {
		currStatus = strings.ToLower(alloc.ClientStatus)
		counts[currStatus] += 1
	}

	return counts
}

func jobsByStatus(jobs []*api.JobListStub) map[string]int {
	var counts map[string]int = make(map[string]int)
	var currStatus string

	counts[strings.ToLower("running")] = 0
	counts[strings.ToLower("dead")] = 0
	counts[strings.ToLower("pending")] = 0
	// if there is a job with a status other than these, it seems it will automatically be initialized in the map to 1 via the +=.

	for _, job := range jobs {
		currStatus = strings.ToLower(job.Status)
		counts[currStatus] += 1
	}

	return counts
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
