package collector

import (
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type BeegfsCollector struct {
	cfg Config

	serverRequests      *prometheus.Desc
	serverQueueLength   *prometheus.Desc
	serverBusyWorkers   *prometheus.Desc
	serverReadBytes     *prometheus.Desc
	serverWrittenBytes  *prometheus.Desc
	serverSentBytes     *prometheus.Desc
	serverReceivedBytes *prometheus.Desc

	targetSpaceTotal       *prometheus.Desc
	targetSpaceFree        *prometheus.Desc
	targetInodesTotal      *prometheus.Desc
	targetInodesFree       *prometheus.Desc
	targetReachable        *prometheus.Desc
	targetConsistencyState *prometheus.Desc

	clientOps *prometheus.Desc

	scrapeDuration *prometheus.Desc
	scrapeSuccess  *prometheus.Desc

	mu    sync.Mutex
	cache *scrapeCache
}

type scrapeCache struct {
	servers   []ServerStats
	targets   []TargetStats
	clientOps []ClientOpStats
	timestamp time.Time
}

func New(cfg Config) *BeegfsCollector {
	ns := "beegfs"
	serverLabels := []string{"node_type", "node_id", "alias"}
	targetLabels := []string{"node_type", "target_id", "node_id", "alias"}
	clientLabels := []string{"node_type", "client", "operation"}

	return &BeegfsCollector{
		cfg: cfg,

		serverRequests:      prometheus.NewDesc(ns+"_server_requests", "Server requests per second", serverLabels, nil),
		serverQueueLength:   prometheus.NewDesc(ns+"_server_queue_length", "Server pending request queue length", serverLabels, nil),
		serverBusyWorkers:   prometheus.NewDesc(ns+"_server_busy_workers", "Server busy worker threads", serverLabels, nil),
		serverReadBytes:     prometheus.NewDesc(ns+"_server_read_bytes", "Server read bytes per second", serverLabels, nil),
		serverWrittenBytes:  prometheus.NewDesc(ns+"_server_written_bytes", "Server written bytes per second", serverLabels, nil),
		serverSentBytes:     prometheus.NewDesc(ns+"_server_sent_bytes", "Server network sent bytes per second", serverLabels, nil),
		serverReceivedBytes: prometheus.NewDesc(ns+"_server_received_bytes", "Server network received bytes per second", serverLabels, nil),

		targetSpaceTotal:       prometheus.NewDesc(ns+"_target_space_total_bytes", "Target total disk space in bytes", targetLabels, nil),
		targetSpaceFree:        prometheus.NewDesc(ns+"_target_space_free_bytes", "Target free disk space in bytes", targetLabels, nil),
		targetInodesTotal:      prometheus.NewDesc(ns+"_target_inodes_total", "Target total inodes", targetLabels, nil),
		targetInodesFree:       prometheus.NewDesc(ns+"_target_inodes_free", "Target free inodes", targetLabels, nil),
		targetReachable:        prometheus.NewDesc(ns+"_target_reachable", "Target reachability (1=online, 0=offline)", targetLabels, nil),
		targetConsistencyState: prometheus.NewDesc(ns+"_target_consistency_state", "Target consistency state (0=good, 1=needs_resync, 2=bad)", targetLabels, nil),

		clientOps: prometheus.NewDesc(ns+"_client_ops_total", "Client operations since server start", clientLabels, nil),

		scrapeDuration: prometheus.NewDesc(ns+"_scrape_duration_seconds", "Time spent collecting BeeGFS metrics", nil, nil),
		scrapeSuccess:  prometheus.NewDesc(ns+"_scrape_success", "Whether the last scrape was successful (1=yes, 0=no)", nil, nil),
	}
}

func (c *BeegfsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.serverRequests
	ch <- c.serverQueueLength
	ch <- c.serverBusyWorkers
	ch <- c.serverReadBytes
	ch <- c.serverWrittenBytes
	ch <- c.serverSentBytes
	ch <- c.serverReceivedBytes

	ch <- c.targetSpaceTotal
	ch <- c.targetSpaceFree
	ch <- c.targetInodesTotal
	ch <- c.targetInodesFree
	ch <- c.targetReachable
	ch <- c.targetConsistencyState

	ch <- c.clientOps

	ch <- c.scrapeDuration
	ch <- c.scrapeSuccess
}

func (c *BeegfsCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	c.mu.Lock()
	needRefresh := c.cache == nil || time.Since(c.cache.timestamp) > c.cfg.Interval
	c.mu.Unlock()

	if needRefresh {
		c.refresh()
	}

	c.mu.Lock()
	cache := c.cache
	c.mu.Unlock()

	if cache == nil {
		ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds())
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0)
		return
	}

	for _, s := range cache.servers {
		labels := []string{s.NodeType, s.NodeID, s.Alias}
		ch <- prometheus.MustNewConstMetric(c.serverRequests, prometheus.GaugeValue, float64(s.Requests), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverQueueLength, prometheus.GaugeValue, float64(s.QueueLength), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverBusyWorkers, prometheus.GaugeValue, float64(s.BusyWorkers), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverReadBytes, prometheus.GaugeValue, float64(s.ReadBytes), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverWrittenBytes, prometheus.GaugeValue, float64(s.WrittenBytes), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverSentBytes, prometheus.GaugeValue, float64(s.SentBytes), labels...)
		ch <- prometheus.MustNewConstMetric(c.serverReceivedBytes, prometheus.GaugeValue, float64(s.ReceivedBytes), labels...)
	}

	for _, t := range cache.targets {
		labels := []string{t.NodeType, t.TargetID, t.NodeID, t.Alias}
		ch <- prometheus.MustNewConstMetric(c.targetSpaceTotal, prometheus.GaugeValue, float64(t.SpaceTotal), labels...)
		ch <- prometheus.MustNewConstMetric(c.targetSpaceFree, prometheus.GaugeValue, float64(t.SpaceFree), labels...)
		ch <- prometheus.MustNewConstMetric(c.targetInodesTotal, prometheus.GaugeValue, float64(t.InodesTotal), labels...)
		ch <- prometheus.MustNewConstMetric(c.targetInodesFree, prometheus.GaugeValue, float64(t.InodesFree), labels...)
		ch <- prometheus.MustNewConstMetric(c.targetReachable, prometheus.GaugeValue, float64(t.Reachable), labels...)
		ch <- prometheus.MustNewConstMetric(c.targetConsistencyState, prometheus.GaugeValue, float64(t.ConsistencyState), labels...)
	}

	for _, op := range cache.clientOps {
		ch <- prometheus.MustNewConstMetric(c.clientOps, prometheus.CounterValue, float64(op.Count), op.NodeType, op.Client, op.Operation)
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds())
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1)
}

func (c *BeegfsCollector) refresh() {
	slog.Debug("refreshing beegfs metrics")

	var wg sync.WaitGroup
	var mu sync.Mutex
	newCache := &scrapeCache{timestamp: time.Now()}

	wg.Add(3)

	go func() {
		defer wg.Done()
		stats := collectAllServerStats(c.cfg)
		mu.Lock()
		newCache.servers = stats
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		targets, err := collectTargets(c.cfg)
		if err != nil {
			slog.Error("failed to collect targets", "err", err)
			return
		}
		mu.Lock()
		newCache.targets = targets
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ops := collectAllClientStats(c.cfg)
		mu.Lock()
		newCache.clientOps = ops
		mu.Unlock()
	}()

	wg.Wait()

	c.mu.Lock()
	c.cache = newCache
	c.mu.Unlock()
}
