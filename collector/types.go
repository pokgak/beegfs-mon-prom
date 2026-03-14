package collector

type MetaNodeStats struct {
	NodeID               string
	Hostname             string
	IsResponding         bool
	IndirectWorkQueueSize uint64
	DirectWorkQueueSize  uint64
	SessionCount         uint64
	WorkRequests         uint64
	QueuedRequests       uint64
	NetSendBytes         uint64
	NetRecvBytes         uint64
}

type StorageNodeStats struct {
	NodeID               string
	Hostname             string
	IsResponding         bool
	IndirectWorkQueueSize uint64
	DirectWorkQueueSize  uint64
	DiskSpaceTotalBytes  uint64
	DiskSpaceFreeBytes   uint64
	SessionCount         uint64
	WorkRequests         uint64
	QueuedRequests       uint64
	DiskWriteBytes       uint64
	DiskReadBytes        uint64
	NetSendBytes         uint64
	NetRecvBytes         uint64
}

type StorageTargetStats struct {
	NodeID             string
	TargetID           string
	DiskSpaceTotalBytes uint64
	DiskSpaceFreeBytes uint64
	InodesTotal        uint64
	InodesFree         uint64
	ConsistencyState   int // 0=good, 1=needs_resync, 2=bad
}

type ClientOpStats struct {
	Client    string
	Operation string
	Count     uint64
}
