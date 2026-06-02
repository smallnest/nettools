package config

import (
	"testing"
)

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		input   string
		want    PortRange
		wantErr bool
	}{
		{"43500,43599", PortRange{Min: 43500, Max: 43599}, false},
		{" 43500 , 43599 ", PortRange{Min: 43500, Max: 43599}, false},
		{"1000,1000", PortRange{Min: 1000, Max: 1000}, false},
		{"9999,1", PortRange{}, true},      // min > max
		{"0,100", PortRange{}, true},       // port 0 invalid
		{"1,70000", PortRange{}, true},     // port > 65535
		{"43500", PortRange{}, true},       // single value
		{"43500-43599", PortRange{}, true}, // wrong separator
		{"abc,def", PortRange{}, true},     // non-numeric
		{"", PortRange{}, true},            // empty
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParsePortRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePortRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParsePortRange(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetNextPorts(t *testing.T) {
	cpr := PortRange{Min: 100, Max: 102}
	spr := PortRange{Min: 200, Max: 202}

	tests := []struct {
		clientPort uint16
		serverPort uint16
		wantC      uint16
		wantS      uint16
	}{
		{100, 200, 100, 201},
		{100, 202, 101, 200}, // server wraps, client increments
		{102, 202, 100, 200}, // both wrap
	}
	for _, tt := range tests {
		gotC, gotS := GetNextPorts(tt.clientPort, tt.serverPort, cpr, spr)
		if gotC != tt.wantC || gotS != tt.wantS {
			t.Errorf("GetNextPorts(%d,%d) = (%d,%d), want (%d,%d)",
				tt.clientPort, tt.serverPort, gotC, gotS, tt.wantC, tt.wantS)
		}
	}
}

func TestValidateDefaults(t *testing.T) {
	cfg := &Config{
		Role:       RoleServer,
		ClientAddr: "1.2.3.4",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClientPortRange != (PortRange{Min: 43500, Max: 43599}) {
		t.Errorf("ClientPortRange = %v, want {43500,43599}", cfg.ClientPortRange)
	}
	if cfg.ServerPortRange != (PortRange{Min: 43500, Max: 43509}) {
		t.Errorf("ServerPortRange = %v, want {43500,43509}", cfg.ServerPortRange)
	}
	if cfg.RateInSpan != 5000 {
		t.Errorf("RateInSpan = %d, want 5000", cfg.RateInSpan)
	}
	if cfg.Span.Seconds() != 1 {
		t.Errorf("Span = %v, want 1s", cfg.Span)
	}
	if cfg.Delay.Seconds() != 3 {
		t.Errorf("Delay = %v, want 3s", cfg.Delay)
	}
}

func TestValidateClientMsgLen(t *testing.T) {
	cfg := &Config{
		Role:       RoleClient,
		ClientAddr: "1.2.3.4",
		MsgLen:     0,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for zero MsgLen on client")
	}
}
