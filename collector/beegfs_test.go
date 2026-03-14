package collector

import (
	"encoding/json"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "clean JSON array",
			input: `[{"a":"1"}]`,
			want:  `[{"a":"1"}]`,
		},
		{
			name:  "with prefix",
			input: "------- 0s -------\n[{\"a\":\"1\"}]\n",
			want:  `[{"a":"1"}]`,
		},
		{
			name:  "with suffix warning",
			input: "[{\"a\":\"1\"}]\n\nWARNING: license\n",
			want:  `[{"a":"1"}]`,
		},
		{
			name:  "with both prefix and suffix",
			input: "------- 0s -------\n[{\"a\":\"1\"}]\n\nWARNING: license expired\n",
			want:  `[{"a":"1"}]`,
		},
		{
			name:    "no JSON",
			input:   "no json here",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSON([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("extractJSON() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"0", 0},
		{"1234", 1234},
		{"", 0},
		{"abc", 0},
		{" 42 ", 42},
		{"18446744073709551615", 18446744073709551615}, // max uint64
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseUint(tt.input); got != tt.want {
				t.Errorf("parseUint(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseServerStatsJSON(t *testing.T) {
	input := `[{"alias":"node_meta_1","busy_workers":"2","node_id":"m:1","queue_length":"5","read":"0","received":"1024","requests":"100","sent":"2048","written":"0"},{"alias":"node_storage_1","busy_workers":"4","node_id":"s:1","queue_length":"10","read":"65536","received":"4096","requests":"500","sent":"8192","written":"131072"}]`

	var raw []serverStatsJSON
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}

	if len(raw) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(raw))
	}

	// Verify meta node
	m := raw[0]
	if m.Alias != "node_meta_1" || m.NodeID != "m:1" {
		t.Errorf("unexpected meta node: alias=%q node_id=%q", m.Alias, m.NodeID)
	}
	if parseUint(m.Requests) != 100 {
		t.Errorf("expected requests=100, got %d", parseUint(m.Requests))
	}
	if parseUint(m.QueueLength) != 5 {
		t.Errorf("expected queue_length=5, got %d", parseUint(m.QueueLength))
	}
	if parseUint(m.BusyWorkers) != 2 {
		t.Errorf("expected busy_workers=2, got %d", parseUint(m.BusyWorkers))
	}
	if parseUint(m.Sent) != 2048 {
		t.Errorf("expected sent=2048, got %d", parseUint(m.Sent))
	}
	if parseUint(m.Received) != 1024 {
		t.Errorf("expected received=1024, got %d", parseUint(m.Received))
	}

	// Verify storage node
	s := raw[1]
	if s.Alias != "node_storage_1" || s.NodeID != "s:1" {
		t.Errorf("unexpected storage node: alias=%q node_id=%q", s.Alias, s.NodeID)
	}
	if parseUint(s.Read) != 65536 {
		t.Errorf("expected read=65536, got %d", parseUint(s.Read))
	}
	if parseUint(s.Written) != 131072 {
		t.Errorf("expected written=131072, got %d", parseUint(s.Written))
	}
}

func TestParseTargetJSON(t *testing.T) {
	input := `[{"alias":"target_meta_1","cap_pool":"Normal","consistency":"Good","id":{"num_id":1,"node_type":2},"inodes":"468840448","inodes_free":"462438162","last_contact":"4s ago","node":"m:1","reachability":"Online","space":"30602483335168","space_free":"5632914296832","storage_pool":"(n/a)","sync_state":"Healthy","type":2},{"alias":"target_storage_1","cap_pool":"Normal","consistency":"Good","id":{"num_id":1,"node_type":3},"inodes":"3000569920","inodes_free":"3000569763","last_contact":"2s ago","node":"s:1","reachability":"Online","space":"30723699376128","space_free":"30508981305344","storage_pool":"s:1","sync_state":"Healthy","type":3}]`

	var raw []targetJSON
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatal(err)
	}

	if len(raw) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(raw))
	}

	// Meta target
	mt := raw[0]
	if mt.Type != 2 {
		t.Errorf("expected type=2 (meta), got %d", mt.Type)
	}
	if mt.Reachability != "Online" {
		t.Errorf("expected reachability=Online, got %q", mt.Reachability)
	}
	if mt.Consistency != "Good" {
		t.Errorf("expected consistency=Good, got %q", mt.Consistency)
	}
	if parseUint(mt.Space) != 30602483335168 {
		t.Errorf("expected space=30602483335168, got %d", parseUint(mt.Space))
	}
	if parseUint(mt.Inodes) != 468840448 {
		t.Errorf("expected inodes=468840448, got %d", parseUint(mt.Inodes))
	}

	// Storage target
	st := raw[1]
	if st.Type != 3 {
		t.Errorf("expected type=3 (storage), got %d", st.Type)
	}
	if st.Node != "s:1" {
		t.Errorf("expected node=s:1, got %q", st.Node)
	}
}

func TestParseTargetConsistencyStates(t *testing.T) {
	tests := []struct {
		consistency string
		want        int
	}{
		{"Good", 0},
		{"good", 0},
		{"Needs_Resync", 1},
		{"needs_resync", 1},
		{"Bad", 2},
		{"Unknown", 2},
	}

	for _, tt := range tests {
		t.Run(tt.consistency, func(t *testing.T) {
			input := `[{"alias":"t","consistency":"` + tt.consistency + `","id":{"num_id":1,"node_type":3},"node":"s:1","reachability":"Online","space":"0","space_free":"0","inodes":"0","inodes_free":"0","type":3}]`

			var raw []targetJSON
			if err := json.Unmarshal([]byte(input), &raw); err != nil {
				t.Fatal(err)
			}

			r := raw[0]
			consistency := 0
			switch {
			case r.Consistency == "" || equalsIgnoreCase(r.Consistency, "good"):
				consistency = 0
			case equalsIgnoreCase(r.Consistency, "needs_resync"):
				consistency = 1
			default:
				consistency = 2
			}

			if consistency != tt.want {
				t.Errorf("consistency(%q) = %d, want %d", tt.consistency, consistency, tt.want)
			}
		})
	}
}

func equalsIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestParseClientStatsJSON(t *testing.T) {
	input := `------- 0s -------
[{"ack":"","client":"10.20.0.7","close":"","mdsinf":"28","schdrct":"2","sum":"30"},{"ack":"4","client":"::","close":"","sum":"4"}]`

	jsonData, err := extractJSON([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var raw []map[string]string
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatal(err)
	}

	if len(raw) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(raw))
	}

	// First entry: real client
	entry := raw[0]
	if entry["client"] != "10.20.0.7" {
		t.Errorf("expected client=10.20.0.7, got %q", entry["client"])
	}
	if parseUint(entry["mdsinf"]) != 28 {
		t.Errorf("expected mdsinf=28, got %d", parseUint(entry["mdsinf"]))
	}
	if parseUint(entry["schdrct"]) != 2 {
		t.Errorf("expected schdrct=2, got %d", parseUint(entry["schdrct"]))
	}
	if parseUint(entry["close"]) != 0 {
		t.Errorf("expected close=0 (empty string), got %d", parseUint(entry["close"]))
	}

	// Second entry: internal (::) should be skipped
	if raw[1]["client"] != "::" {
		t.Errorf("expected second client=::, got %q", raw[1]["client"])
	}
}

func TestParseStorageClientStatsJSON(t *testing.T) {
	input := `------- 0s -------
[{"ack":"","client":"10.20.0.64","ops-rd":"2556359","ops-wr":"3367924","rd":"428625104896","sum":"5924461","wr":"551462342656"}]`

	jsonData, err := extractJSON([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var raw []map[string]string
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatal(err)
	}

	entry := raw[0]
	if parseUint(entry["ops-rd"]) != 2556359 {
		t.Errorf("expected ops-rd=2556359, got %d", parseUint(entry["ops-rd"]))
	}
	if parseUint(entry["rd"]) != 428625104896 {
		t.Errorf("expected rd=428625104896, got %d", parseUint(entry["rd"]))
	}
	if parseUint(entry["wr"]) != 551462342656 {
		t.Errorf("expected wr=551462342656, got %d", parseUint(entry["wr"]))
	}
}
