package collector

import (
	"testing"
)

func TestParseByteValue(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
		wantErr  bool
	}{
		{"1024", 1024, false},
		{"10GiB", 10 * 1024 * 1024 * 1024, false},
		{"5TiB", 5 * 1024 * 1024 * 1024 * 1024, false},
		{"100MiB", 100 * 1024 * 1024, false},
		{"512KiB", 512 * 1024, false},
		{"1000B", 1000, false},
		{"10GB", 10 * 1000 * 1000 * 1000, false},
		{"-", 0, true},
		{"N/A", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseByteValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseByteValue(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseByteValue(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseNodeList(t *testing.T) {
	input := `meta01 [ID: 1]
meta02 [ID: 2]
`
	nodes := parseNodeList([]byte(input), true)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].nodeID != "1" || nodes[0].hostname != "meta01" || !nodes[0].reachable {
		t.Errorf("unexpected node[0]: %+v", nodes[0])
	}
	if nodes[1].nodeID != "2" || nodes[1].hostname != "meta02" {
		t.Errorf("unexpected node[1]: %+v", nodes[1])
	}
}
