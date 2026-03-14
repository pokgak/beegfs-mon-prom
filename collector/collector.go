package collector

import (
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	CfgFile  string
	MgmtHost string
	Interval time.Duration
}

type BeegfsCollector struct {
	cfg Config

	// meta node metrics
	metaResponding         *prometheus.Desc
	metaIndirectWorkQueue  *prometheus.Desc
	metaDirectWorkQueue    *prometheus.Desc
	metaSessions           *prometheus.Desc
	metaWorkRequests       *prometheus.Desc
	metaQueuedRequests     *prometheus.Desc
	metaNetSendBytes       *prometheus.Desc
	metaNetRecvBytes       *prometheus.Desc

	// storage node metrics
	storageResponding        *prometheus.Desc
	storageIndirectWorkQueue *prometheus.Desc
	storageDirectWorkQueue   *prometheus.Desc
	storageDiskSpaceTotal    *prometheus.Desc
	storageDiskSpaceFree     *prometheus.Desc
	storageSessions          *prometheus.Desc
	storageWorkRequests      *prometheus.Desc
	storageQueuedRequests    *prometheus.Desc
	storageDiskWriteBytes    *prometheus.Desc
	storageDiskReadBytes     *prometheus.Desc
	storageNetSendBytes      *prometheus.Desc
	storageNetRecvBytes      *prometheus.Desc

	// storage target metrics
	targetDiskSpaceTotal    *prometheus.Desc
	targetDiskSpaceFree     *prometheus.Desc
	targetInodesTotal       *prometheus.Desc
	targetInodesFree        *prometheus.Desc
	targetConsistencyState  *prometheus.Desc

	// client ops metrics
	clientMetaOps    *prometheus.Desc
	clientStorageOps *prometheus.Desc

	// scrape metadata
	scrapeDuration *prometheus.Desc
	scrapeSuccess  *prometheus.Desc

	mu      sync.Mutex
	cache   *scrapeCache
}

type scrapeCache struct {
	metaNodes      []MetaNodeStats
	storageNodes   []StorageNodeStats
	storageTargets []StorageTargetStats
	clientMetaOps  []ClientOpStats
	clientStorOps  []ClientOpStats
	timestamp      time.Time
}

func New(cfg Config) *BeegfsCollector {
	ns := "beegfs"

	return &BeegfsCollector{
		cfg: cfg,

		metaResponding:        prometheus.NewDesc(ns+"_meta_responding", "Whether the meta node is responding (1=yes, 0=no)", []string{"node_id", "hostname"}, nil),
		metaIndirectWorkQueue: prometheus.NewDesc(ns+"_meta_indirect_work_queue_size", "Meta node indirect work queue size", []string{"node_id", "hostname"}, nil),
		metaDirectWorkQueue:   prometheus.NewDesc(ns+"_meta_direct_work_queue_size", "Meta node direct work queue size", []string{"node_id", "hostname"}, nil),
		metaSessions:          prometheus.NewDesc(ns+"_meta_sessions", "Meta node session count", []string{"node_id", "hostname"}, nil),
		metaWorkRequests:      prometheus.NewDesc(ns+"_meta_work_requests_total", "Meta node work requests (high-res)", []string{"node_id", "hostname"}, nil),
		metaQueuedRequests:    prometheus.NewDesc(ns+"_meta_queued_requests", "Meta node queued requests (high-res)", []string{"node_id", "hostname"}, nil),
		metaNetSendBytes:      prometheus.NewDesc(ns+"_meta_net_send_bytes_total", "Meta node network bytes sent (high-res)", []string{"node_id", "hostname"}, nil),
		metaNetRecvBytes:      prometheus.NewDesc(ns+"_meta_net_recv_bytes_total", "Meta node network bytes received (high-res)", []string{"node_id", "hostname"}, nil),

		storageResponding:        prometheus.NewDesc(ns+"_storage_responding", "Whether the storage node is responding (1=yes, 0=no)", []string{"node_id", "hostname"}, nil),
		storageIndirectWorkQueue: prometheus.NewDesc(ns+"_storage_indirect_work_queue_size", "Storage node indirect work queue size", []string{"node_id", "hostname"}, nil),
		storageDirectWorkQueue:   prometheus.NewDesc(ns+"_storage_direct_work_queue_size", "Storage node direct work queue size", []string{"node_id", "hostname"}, nil),
		storageDiskSpaceTotal:    prometheus.NewDesc(ns+"_storage_disk_space_total_bytes", "Storage node total disk space in bytes", []string{"node_id", "hostname"}, nil),
		storageDiskSpaceFree:     prometheus.NewDesc(ns+"_storage_disk_space_free_bytes", "Storage node free disk space in bytes", []string{"node_id", "hostname"}, nil),
		storageSessions:          prometheus.NewDesc(ns+"_storage_sessions", "Storage node session count", []string{"node_id", "hostname"}, nil),
		storageWorkRequests:      prometheus.NewDesc(ns+"_storage_work_requests_total", "Storage node work requests (high-res)", []string{"node_id", "hostname"}, nil),
		storageQueuedRequests:    prometheus.NewDesc(ns+"_storage_queued_requests", "Storage node queued requests (high-res)", []string{"node_id", "hostname"}, nil),
		storageDiskWriteBytes:    prometheus.NewDesc(ns+"_storage_disk_write_bytes_total", "Storage node disk write bytes (high-res)", []string{"node_id", "hostname"}, nil),
		storageDiskReadBytes:     prometheus.NewDesc(ns+"_storage_disk_read_bytes_total", "Storage node disk read bytes (high-res)", []string{"node_id", "hostname"}, nil),
		storageNetSendBytes:      prometheus.NewDesc(ns+"_storage_net_send_bytes_total", "Storage node network bytes sent (high-res)", []string{"node_id", "hostname"}, nil),
		storageNetRecvBytes:      prometheus.NewDesc(ns+"_storage_net_recv_bytes_total", "Storage node network bytes received (high-res)", []string{"node_id", "hostname"}, nil),

		targetDiskSpaceTotal:   prometheus.NewDesc(ns+"_target_disk_space_total_bytes", "Storage target total disk space in bytes", []string{"node_id", "target_id"}, nil),
		targetDiskSpaceFree:    prometheus.NewDesc(ns+"_target_disk_space_free_bytes", "Storage target free disk space in bytes", []string{"node_id", "target_id"}, nil),
		targetInodesTotal:      prometheus.NewDesc(ns+"_target_inodes_total", "Storage target total inodes", []string{"node_id", "target_id"}, nil),
		targetInodesFree:       prometheus.NewDesc(ns+"_target_inodes_free", "Storage target free inodes", []string{"node_id", "target_id"}, nil),
		targetConsistencyState: prometheus.NewDesc(ns+"_target_consistency_state", "Storage target consistency state (0=good, 1=needs_resync, 2=bad)", []string{"node_id", "target_id"}, nil),

		clientMetaOps:    prometheus.NewDesc(ns+"_client_meta_ops_total", "Client metadata operations", []string{"client", "operation"}, nil),
		clientStorageOps: prometheus.NewDesc(ns+"_client_storage_ops_total", "Client storage operations", []string{"client", "operation"}, nil),

		scrapeDuration: prometheus.NewDesc(ns+"_scrape_duration_seconds", "Time spent collecting BeeGFS metrics", nil, nil),
		scrapeSuccess:  prometheus.NewDesc(ns+"_scrape_success", "Whether the last scrape was successful (1=yes, 0=no)", nil, nil),
	}
}

func (c *BeegfsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metaResponding
	ch <- c.metaIndirectWorkQueue
	ch <- c.metaDirectWorkQueue
	ch <- c.metaSessions
	ch <- c.metaWorkRequests
	ch <- c.metaQueuedRequests
	ch <- c.metaNetSendBytes
	ch <- c.metaNetRecvBytes

	ch <- c.storageResponding
	ch <- c.storageIndirectWorkQueue
	ch <- c.storageDirectWorkQueue
	ch <- c.storageDiskSpaceTotal
	ch <- c.storageDiskSpaceFree
	ch <- c.storageSessions
	ch <- c.storageWorkRequests
	ch <- c.storageQueuedRequests
	ch <- c.storageDiskWriteBytes
	ch <- c.storageDiskReadBytes
	ch <- c.storageNetSendBytes
	ch <- c.storageNetRecvBytes

	ch <- c.targetDiskSpaceTotal
	ch <- c.targetDiskSpaceFree
	ch <- c.targetInodesTotal
	ch <- c.targetInodesFree
	ch <- c.targetConsistencyState

	ch <- c.clientMetaOps
	ch <- c.clientStorageOps

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

	for _, m := range cache.metaNodes {
		responding := float64(0)
		if m.IsResponding {
			responding = 1
		}
		ch <- prometheus.MustNewConstMetric(c.metaResponding, prometheus.GaugeValue, responding, m.NodeID, m.Hostname)
		if m.IsResponding {
			ch <- prometheus.MustNewConstMetric(c.metaIndirectWorkQueue, prometheus.GaugeValue, float64(m.IndirectWorkQueueSize), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaDirectWorkQueue, prometheus.GaugeValue, float64(m.DirectWorkQueueSize), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaSessions, prometheus.GaugeValue, float64(m.SessionCount), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaWorkRequests, prometheus.GaugeValue, float64(m.WorkRequests), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaQueuedRequests, prometheus.GaugeValue, float64(m.QueuedRequests), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaNetSendBytes, prometheus.GaugeValue, float64(m.NetSendBytes), m.NodeID, m.Hostname)
			ch <- prometheus.MustNewConstMetric(c.metaNetRecvBytes, prometheus.GaugeValue, float64(m.NetRecvBytes), m.NodeID, m.Hostname)
		}
	}

	for _, s := range cache.storageNodes {
		responding := float64(0)
		if s.IsResponding {
			responding = 1
		}
		ch <- prometheus.MustNewConstMetric(c.storageResponding, prometheus.GaugeValue, responding, s.NodeID, s.Hostname)
		if s.IsResponding {
			ch <- prometheus.MustNewConstMetric(c.storageIndirectWorkQueue, prometheus.GaugeValue, float64(s.IndirectWorkQueueSize), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageDirectWorkQueue, prometheus.GaugeValue, float64(s.DirectWorkQueueSize), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageDiskSpaceTotal, prometheus.GaugeValue, float64(s.DiskSpaceTotalBytes), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageDiskSpaceFree, prometheus.GaugeValue, float64(s.DiskSpaceFreeBytes), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageSessions, prometheus.GaugeValue, float64(s.SessionCount), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageWorkRequests, prometheus.GaugeValue, float64(s.WorkRequests), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageQueuedRequests, prometheus.GaugeValue, float64(s.QueuedRequests), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageDiskWriteBytes, prometheus.GaugeValue, float64(s.DiskWriteBytes), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageDiskReadBytes, prometheus.GaugeValue, float64(s.DiskReadBytes), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageNetSendBytes, prometheus.GaugeValue, float64(s.NetSendBytes), s.NodeID, s.Hostname)
			ch <- prometheus.MustNewConstMetric(c.storageNetRecvBytes, prometheus.GaugeValue, float64(s.NetRecvBytes), s.NodeID, s.Hostname)
		}
	}

	for _, t := range cache.storageTargets {
		ch <- prometheus.MustNewConstMetric(c.targetDiskSpaceTotal, prometheus.GaugeValue, float64(t.DiskSpaceTotalBytes), t.NodeID, t.TargetID)
		ch <- prometheus.MustNewConstMetric(c.targetDiskSpaceFree, prometheus.GaugeValue, float64(t.DiskSpaceFreeBytes), t.NodeID, t.TargetID)
		ch <- prometheus.MustNewConstMetric(c.targetInodesTotal, prometheus.GaugeValue, float64(t.InodesTotal), t.NodeID, t.TargetID)
		ch <- prometheus.MustNewConstMetric(c.targetInodesFree, prometheus.GaugeValue, float64(t.InodesFree), t.NodeID, t.TargetID)
		ch <- prometheus.MustNewConstMetric(c.targetConsistencyState, prometheus.GaugeValue, float64(t.ConsistencyState), t.NodeID, t.TargetID)
	}

	for _, op := range cache.clientMetaOps {
		ch <- prometheus.MustNewConstMetric(c.clientMetaOps, prometheus.GaugeValue, float64(op.Count), op.Client, op.Operation)
	}

	for _, op := range cache.clientStorOps {
		ch <- prometheus.MustNewConstMetric(c.clientStorageOps, prometheus.GaugeValue, float64(op.Count), op.Client, op.Operation)
	}

	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds())
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1)
}

func (c *BeegfsCollector) refresh() {
	slog.Debug("refreshing beegfs metrics")

	var wg sync.WaitGroup
	var mu sync.Mutex
	newCache := &scrapeCache{timestamp: time.Now()}

	ctlArgs := c.ctlBaseArgs()

	wg.Add(4)

	go func() {
		defer wg.Done()
		nodes, err := collectMetaNodes(ctlArgs)
		if err != nil {
			slog.Error("failed to collect meta nodes", "err", err)
			return
		}
		mu.Lock()
		newCache.metaNodes = nodes
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		nodes, err := collectStorageNodes(ctlArgs)
		if err != nil {
			slog.Error("failed to collect storage nodes", "err", err)
			return
		}
		targets, err := collectStorageTargets(ctlArgs)
		if err != nil {
			slog.Error("failed to collect storage targets", "err", err)
		}
		mu.Lock()
		newCache.storageNodes = nodes
		newCache.storageTargets = targets
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ops, err := collectClientStats(ctlArgs, "meta")
		if err != nil {
			slog.Error("failed to collect client meta ops", "err", err)
			return
		}
		mu.Lock()
		newCache.clientMetaOps = ops
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ops, err := collectClientStats(ctlArgs, "storage")
		if err != nil {
			slog.Error("failed to collect client storage ops", "err", err)
			return
		}
		mu.Lock()
		newCache.clientStorOps = ops
		mu.Unlock()
	}()

	wg.Wait()

	c.mu.Lock()
	c.cache = newCache
	c.mu.Unlock()
}

func (c *BeegfsCollector) ctlBaseArgs() []string {
	var args []string
	if c.cfg.CfgFile != "" {
		args = append(args, "--cfgFile="+c.cfg.CfgFile)
	}
	if c.cfg.MgmtHost != "" {
		args = append(args, "--sysMgmtdHost="+c.cfg.MgmtHost)
	}
	return args
}
