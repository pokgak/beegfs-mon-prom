package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

func (cfg Config) baseArgs() []string {
	var args []string
	if cfg.TLSDisable {
		args = append(args, "--tls-disable")
	}
	if cfg.TLSCertFile != "" {
		args = append(args, "--tls-cert-file", cfg.TLSCertFile)
	}
	if cfg.AuthFile != "" {
		args = append(args, "--auth-file", cfg.AuthFile)
	}
	if cfg.MgmtdAddr != "" {
		args = append(args, "--mgmtd-addr", cfg.MgmtdAddr)
	}
	return args
}

func runBeegfs(cfg Config, extraArgs ...string) ([]byte, error) {
	args := append(cfg.baseArgs(), extraArgs...)
	cmd := exec.Command(cfg.BeegfsPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %v failed: %w: %s", cfg.BeegfsPath, extraArgs, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// extractJSON finds the first JSON array in the output, handling
// non-JSON prefixes like "------- 0s -------" from stats client.
func extractJSON(data []byte) ([]byte, error) {
	idx := bytes.IndexByte(data, '[')
	if idx < 0 {
		return nil, fmt.Errorf("no JSON array found in output")
	}
	end := bytes.LastIndexByte(data, ']')
	if end < idx {
		return nil, fmt.Errorf("malformed JSON array in output")
	}
	return data[idx : end+1], nil
}

func parseUint(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func collectServerStats(cfg Config, nodeType string) ([]ServerStats, error) {
	out, err := runBeegfs(cfg, "stats", "server",
		"--node-type", nodeType,
		"--interval", "0",
		"--history", "0s",
		"--output", "json",
		"--raw",
	)
	if err != nil {
		return nil, err
	}

	jsonData, err := extractJSON(out)
	if err != nil {
		return nil, err
	}

	var raw []serverStatsJSON
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("parsing stats server JSON: %w", err)
	}

	results := make([]ServerStats, len(raw))
	for i, r := range raw {
		results[i] = ServerStats{
			NodeID:        r.NodeID,
			Alias:         r.Alias,
			NodeType:      nodeType,
			Requests:      parseUint(r.Requests),
			QueueLength:   parseUint(r.QueueLength),
			BusyWorkers:   parseUint(r.BusyWorkers),
			ReadBytes:     parseUint(r.Read),
			WrittenBytes:  parseUint(r.Written),
			SentBytes:     parseUint(r.Sent),
			ReceivedBytes: parseUint(r.Received),
		}
	}
	return results, nil
}

func collectTargets(cfg Config) ([]TargetStats, error) {
	out, err := runBeegfs(cfg, "target", "list",
		"--state",
		"--capacity",
		"--output", "json",
		"--raw",
	)
	if err != nil {
		return nil, err
	}

	jsonData, err := extractJSON(out)
	if err != nil {
		return nil, err
	}

	var raw []targetJSON
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("parsing target list JSON: %w", err)
	}

	var results []TargetStats
	for _, r := range raw {
		nodeType := "storage"
		if r.Type == 2 {
			nodeType = "meta"
		}

		reachable := 0
		if strings.EqualFold(r.Reachability, "Online") {
			reachable = 1
		}

		consistency := 0
		switch strings.ToLower(r.Consistency) {
		case "good":
			consistency = 0
		case "needs_resync":
			consistency = 1
		default:
			if r.Consistency != "" && !strings.EqualFold(r.Consistency, "good") {
				consistency = 2
			}
		}

		results = append(results, TargetStats{
			Alias:            r.Alias,
			TargetID:         fmt.Sprintf("%d", r.ID.NumID),
			NodeID:           r.Node,
			NodeType:         nodeType,
			Reachable:        reachable,
			ConsistencyState: consistency,
			SpaceTotal:       parseUint(r.Space),
			SpaceFree:        parseUint(r.SpaceFree),
			InodesTotal:      parseUint(r.Inodes),
			InodesFree:       parseUint(r.InodesFree),
		})
	}
	return results, nil
}

func collectClientStats(cfg Config, nodeType string) ([]ClientOpStats, error) {
	out, err := runBeegfs(cfg, "stats", "client",
		"--node-type", nodeType,
		"--interval", "0",
		"--output", "json",
		"--raw",
	)
	if err != nil {
		return nil, err
	}

	jsonData, err := extractJSON(out)
	if err != nil {
		return nil, err
	}

	var raw []map[string]string
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("parsing stats client JSON: %w", err)
	}

	var results []ClientOpStats
	for _, entry := range raw {
		client := entry["client"]
		if client == "" || client == "::" {
			continue
		}

		for op, val := range entry {
			if op == "client" || op == "sum" {
				continue
			}
			count := parseUint(val)
			if count == 0 {
				continue
			}
			results = append(results, ClientOpStats{
				Client:    client,
				Operation: op,
				NodeType:  nodeType,
				Count:     count,
			})
		}
	}
	return results, nil
}

func collectAllServerStats(cfg Config) []ServerStats {
	var all []ServerStats
	for _, nt := range []string{"meta", "storage"} {
		stats, err := collectServerStats(cfg, nt)
		if err != nil {
			slog.Error("failed to collect server stats", "node_type", nt, "err", err)
			continue
		}
		all = append(all, stats...)
	}
	return all
}

func collectAllClientStats(cfg Config) []ClientOpStats {
	var all []ClientOpStats
	for _, nt := range []string{"meta", "storage"} {
		stats, err := collectClientStats(cfg, nt)
		if err != nil {
			slog.Error("failed to collect client stats", "node_type", nt, "err", err)
			continue
		}
		all = append(all, stats...)
	}
	return all
}
