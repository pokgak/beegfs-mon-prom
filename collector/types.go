package collector

import "time"

type Config struct {
	BeegfsPath  string
	MgmtdAddr   string
	AuthFile    string
	TLSDisable  bool
	TLSCertFile string
	Interval    time.Duration
}

// JSON types from BeeGFS 8.x CLI

type serverStatsJSON struct {
	Alias       string `json:"alias"`
	NodeID      string `json:"node_id"`
	Requests    string `json:"requests"`
	QueueLength string `json:"queue_length"`
	BusyWorkers string `json:"busy_workers"`
	Read        string `json:"read"`
	Written     string `json:"written"`
	Sent        string `json:"sent"`
	Received    string `json:"received"`
}

type targetJSON struct {
	Alias        string `json:"alias"`
	ID           struct {
		NumID    int `json:"num_id"`
		NodeType int `json:"node_type"`
	} `json:"id"`
	Node         string `json:"node"`
	Reachability string `json:"reachability"`
	Consistency  string `json:"consistency"`
	SyncState    string `json:"sync_state"`
	Space        string `json:"space"`
	SpaceFree    string `json:"space_free"`
	Inodes       string `json:"inodes"`
	InodesFree   string `json:"inodes_free"`
	CapPool      string `json:"cap_pool"`
	StoragePool  string `json:"storage_pool"`
	Type         int    `json:"type"`
}

// Internal metric types

type ServerStats struct {
	NodeID        string
	Alias         string
	NodeType      string // "meta" or "storage"
	Requests      uint64
	QueueLength   uint64
	BusyWorkers   uint64
	ReadBytes     uint64
	WrittenBytes  uint64
	SentBytes     uint64
	ReceivedBytes uint64
}

type TargetStats struct {
	Alias            string
	TargetID         string
	NodeID           string
	NodeType         string // "meta" or "storage"
	Reachable        int    // 1=online, 0=offline
	ConsistencyState int    // 0=good, 1=needs_resync, 2=bad
	SpaceTotal       uint64
	SpaceFree        uint64
	InodesTotal      uint64
	InodesFree       uint64
}

type ClientOpStats struct {
	Client    string
	Operation string
	NodeType  string // "meta" or "storage"
	Count     uint64
}
