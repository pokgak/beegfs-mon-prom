package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

const beegfsCtl = "beegfs-ctl"

func runCtl(baseArgs []string, extraArgs ...string) ([]byte, error) {
	args := append(baseArgs, extraArgs...)
	cmd := exec.Command(beegfsCtl, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("beegfs-ctl %v failed: %w: %s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// collectMetaNodes runs `beegfs-ctl --listnodes --nodetype=meta --nicdetails`
// and `beegfs-ctl --iostat --nodetype=meta --interval=0` to gather meta node info.
func collectMetaNodes(baseArgs []string) ([]MetaNodeStats, error) {
	nodes, err := listNodes(baseArgs, "meta")
	if err != nil {
		return nil, err
	}

	stats, err := getIOStat(baseArgs, "meta")
	if err != nil {
		slog.Warn("failed to get meta iostat, nodes will have zero IO stats", "err", err)
	}

	var results []MetaNodeStats
	for _, n := range nodes {
		m := MetaNodeStats{
			NodeID:       n.nodeID,
			Hostname:     n.hostname,
			IsResponding: n.reachable,
		}
		if s, ok := stats[n.nodeID]; ok {
			m.WorkRequests = s.workRequests
			m.QueuedRequests = s.queuedRequests
			m.NetSendBytes = s.netSendBytes
			m.NetRecvBytes = s.netRecvBytes
		}
		results = append(results, m)
	}
	return results, nil
}

// collectStorageNodes runs similar commands for storage nodes.
func collectStorageNodes(baseArgs []string) ([]StorageNodeStats, error) {
	nodes, err := listNodes(baseArgs, "storage")
	if err != nil {
		return nil, err
	}

	stats, err := getIOStat(baseArgs, "storage")
	if err != nil {
		slog.Warn("failed to get storage iostat, nodes will have zero IO stats", "err", err)
	}

	spaceByNode, err := getStorageSpace(baseArgs)
	if err != nil {
		slog.Warn("failed to get storage space info", "err", err)
	}

	var results []StorageNodeStats
	for _, n := range nodes {
		s := StorageNodeStats{
			NodeID:       n.nodeID,
			Hostname:     n.hostname,
			IsResponding: n.reachable,
		}
		if io, ok := stats[n.nodeID]; ok {
			s.WorkRequests = io.workRequests
			s.QueuedRequests = io.queuedRequests
			s.DiskReadBytes = io.diskReadBytes
			s.DiskWriteBytes = io.diskWriteBytes
			s.NetSendBytes = io.netSendBytes
			s.NetRecvBytes = io.netRecvBytes
		}
		if sp, ok := spaceByNode[n.nodeID]; ok {
			s.DiskSpaceTotalBytes = sp.totalBytes
			s.DiskSpaceFreeBytes = sp.freeBytes
		}
		results = append(results, s)
	}
	return results, nil
}

// collectStorageTargets runs `beegfs-ctl --listtargets --longnodes --state --pools`
func collectStorageTargets(baseArgs []string) ([]StorageTargetStats, error) {
	out, err := runCtl(baseArgs, "--listtargets", "--longnodes", "--state", "--pools")
	if err != nil {
		return nil, err
	}

	var targets []StorageTargetStats
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "=") || strings.TrimSpace(line) == "" {
			continue
		}
		// Skip header lines
		if strings.Contains(line, "TargetID") || strings.Contains(line, "targetID") {
			continue
		}

		fields := strings.Fields(line)
		// Expected format: TargetID NodeID Pool Reachability Consistency
		// Actual format varies, parse what we can
		if len(fields) < 5 {
			continue
		}

		targetID := fields[0]
		nodeID := fields[1]
		consistency := strings.ToLower(fields[len(fields)-1])

		state := 0
		switch {
		case strings.Contains(consistency, "good"):
			state = 0
		case strings.Contains(consistency, "resync"):
			state = 1
		default:
			state = 2
		}

		targets = append(targets, StorageTargetStats{
			NodeID:           nodeID,
			TargetID:         targetID,
			ConsistencyState: state,
		})
	}

	// Enrich with space info from listtargets --spaceinfo
	spaceInfo, err := getTargetSpaceInfo(baseArgs)
	if err != nil {
		slog.Warn("failed to get target space info", "err", err)
		return targets, nil
	}

	for i := range targets {
		if si, ok := spaceInfo[targets[i].TargetID]; ok {
			targets[i].DiskSpaceTotalBytes = si.totalBytes
			targets[i].DiskSpaceFreeBytes = si.freeBytes
			targets[i].InodesTotal = si.inodesTotal
			targets[i].InodesFree = si.inodesFree
		}
	}

	return targets, nil
}

// collectClientStats runs `beegfs-ctl --clientstats --nodetype=<meta|storage> --interval=0`
func collectClientStats(baseArgs []string, nodeType string) ([]ClientOpStats, error) {
	out, err := runCtl(baseArgs, "--clientstats", "--nodetype="+nodeType, "--interval=0")
	if err != nil {
		return nil, err
	}

	var results []ClientOpStats
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var headers []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "=") {
			continue
		}

		// Detect header line (contains operation names)
		if strings.Contains(line, "sum") && (strings.Contains(line, "rd") || strings.Contains(line, "open") || strings.Contains(line, "mkdir")) {
			headers = strings.Fields(line)
			continue
		}

		if len(headers) == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Last field is the client IP/hostname, rest are op counts matching headers
		client := fields[len(fields)-1]
		opValues := fields[:len(fields)-1]

		for i, val := range opValues {
			if i >= len(headers) {
				break
			}
			count, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				continue
			}
			if count == 0 {
				continue
			}
			results = append(results, ClientOpStats{
				Client:    client,
				Operation: headers[i],
				Count:     count,
			})
		}
	}

	return results, nil
}

type nodeInfo struct {
	nodeID    string
	hostname  string
	reachable bool
}

// listNodes runs `beegfs-ctl --listnodes --nodetype=<type> --reachable --unreachable`
func listNodes(baseArgs []string, nodeType string) ([]nodeInfo, error) {
	// Get reachable nodes
	reachOut, err := runCtl(baseArgs, "--listnodes", "--nodetype="+nodeType, "--reachable")
	if err != nil {
		return nil, err
	}
	reachable := parseNodeList(reachOut, true)

	// Get unreachable nodes
	unrOut, err := runCtl(baseArgs, "--listnodes", "--nodetype="+nodeType, "--unreachable")
	if err != nil {
		slog.Warn("failed to get unreachable nodes", "nodetype", nodeType, "err", err)
		return reachable, nil
	}
	unreachable := parseNodeList(unrOut, false)

	return append(reachable, unreachable...), nil
}

func parseNodeList(data []byte, reachable bool) []nodeInfo {
	var nodes []nodeInfo
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "=") {
			continue
		}
		// Format: "hostname [ID: nodeID]"
		parts := strings.SplitN(line, "[", 2)
		if len(parts) < 2 {
			continue
		}
		hostname := strings.TrimSpace(parts[0])
		idPart := strings.TrimSuffix(strings.TrimSpace(parts[1]), "]")
		idPart = strings.TrimPrefix(idPart, "ID: ")
		nodeID := strings.TrimSpace(idPart)

		nodes = append(nodes, nodeInfo{
			nodeID:    nodeID,
			hostname:  hostname,
			reachable: reachable,
		})
	}
	return nodes
}

type ioStats struct {
	workRequests  uint64
	queuedRequests uint64
	diskReadBytes  uint64
	diskWriteBytes uint64
	netSendBytes   uint64
	netRecvBytes   uint64
}

// getIOStat runs `beegfs-ctl --iostat --nodetype=<type> --interval=0`
func getIOStat(baseArgs []string, nodeType string) (map[string]ioStats, error) {
	out, err := runCtl(baseArgs, "--iostat", "--nodetype="+nodeType, "--interval=0")
	if err != nil {
		return nil, err
	}

	results := make(map[string]ioStats)
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var headers []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "=") {
			continue
		}

		// Header line detection
		if strings.Contains(line, "Node") && (strings.Contains(line, "Read") || strings.Contains(line, "Work")) {
			headers = strings.Fields(line)
			continue
		}

		if len(headers) == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		nodeID := fields[0]
		st := ioStats{}

		for i, h := range headers {
			if i >= len(fields) {
				break
			}
			val, err := parseByteValue(fields[i])
			if err != nil {
				continue
			}
			h = strings.ToLower(h)
			switch {
			case strings.Contains(h, "read") && strings.Contains(h, "byte"):
				st.diskReadBytes = val
			case strings.Contains(h, "write") && strings.Contains(h, "byte"):
				st.diskWriteBytes = val
			case strings.Contains(h, "send"):
				st.netSendBytes = val
			case strings.Contains(h, "recv"):
				st.netRecvBytes = val
			case strings.Contains(h, "work") || strings.Contains(h, "req"):
				st.workRequests = val
			case strings.Contains(h, "queue"):
				st.queuedRequests = val
			}
		}

		results[nodeID] = st
	}
	return results, nil
}

type spaceInfo struct {
	totalBytes  uint64
	freeBytes   uint64
	inodesTotal uint64
	inodesFree  uint64
}

// getStorageSpace runs `beegfs-ctl --getquota --storagepoolid=default --csv` or similar
// to get aggregate storage space per node. Falls back to listtargets.
func getStorageSpace(baseArgs []string) (map[string]spaceInfo, error) {
	return getTargetSpaceByNode(baseArgs)
}

// getTargetSpaceInfo runs `beegfs-ctl --listtargets --spaceinfo --longnodes`
func getTargetSpaceInfo(baseArgs []string) (map[string]spaceInfo, error) {
	out, err := runCtl(baseArgs, "--listtargets", "--spaceinfo", "--longnodes")
	if err != nil {
		return nil, err
	}

	results := make(map[string]spaceInfo)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "=") {
			continue
		}
		if strings.Contains(line, "TargetID") || strings.Contains(line, "targetID") {
			continue
		}

		fields := strings.Fields(line)
		// Format: TargetID NodeID Total Free InodesTotal InodesFree
		if len(fields) < 4 {
			continue
		}

		targetID := fields[0]
		si := spaceInfo{}

		// Try to parse space fields — they may have units like "100GiB"
		for i := 2; i < len(fields); i++ {
			val, err := parseByteValue(fields[i])
			if err != nil {
				continue
			}
			switch {
			case si.totalBytes == 0 && val > 0:
				si.totalBytes = val
			case si.freeBytes == 0 && val > 0:
				si.freeBytes = val
			}
		}

		results[targetID] = si
	}
	return results, nil
}

// getTargetSpaceByNode aggregates target space per node
func getTargetSpaceByNode(baseArgs []string) (map[string]spaceInfo, error) {
	out, err := runCtl(baseArgs, "--listtargets", "--spaceinfo", "--longnodes")
	if err != nil {
		return nil, err
	}

	results := make(map[string]spaceInfo)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "=") {
			continue
		}
		if strings.Contains(line, "TargetID") || strings.Contains(line, "targetID") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		nodeID := fields[1]
		existing := results[nodeID]

		for i := 2; i < len(fields); i++ {
			val, err := parseByteValue(fields[i])
			if err != nil {
				continue
			}
			switch i {
			case 2:
				existing.totalBytes += val
			case 3:
				existing.freeBytes += val
			case 4:
				existing.inodesTotal += val
			case 5:
				existing.inodesFree += val
			}
		}

		results[nodeID] = existing
	}
	return results, nil
}

// parseByteValue parses values like "100", "10GiB", "5.2TiB", "1024MiB"
func parseByteValue(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "N/A" {
		return 0, fmt.Errorf("empty value")
	}

	multiplier := uint64(1)
	numStr := s

	suffixes := []struct {
		suffix string
		mult   uint64
	}{
		{"TiB", 1024 * 1024 * 1024 * 1024},
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.suffix) {
			numStr = strings.TrimSuffix(s, sf.suffix)
			multiplier = sf.mult
			break
		}
	}

	// Try float first for values like "5.2TiB"
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, err
	}

	return uint64(f * float64(multiplier)), nil
}
