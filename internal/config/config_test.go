package config

import "testing"

func TestPort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		want    int
		wantErr bool
	}{
		{name: "unset falls back to default", port: "", want: DefaultPort},
		{name: "blank falls back to default", port: "   ", want: DefaultPort},
		{name: "numeric port", port: "9090", want: 9090},
		{name: "surrounding whitespace is trimmed", port: "\t8081\n", want: 8081},
		{name: "non-numeric is invalid", port: "http", wantErr: true},
		{name: "float is invalid", port: "80.5", wantErr: true},
		{name: "zero is invalid", port: "0", wantErr: true},
		{name: "negative is invalid", port: "-1", wantErr: true},
		{name: "above TCP range is invalid", port: "65536", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Port(func(string) string { return tt.port })
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Port(%q) = %d, want error", tt.port, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Port(%q) returned unexpected error: %v", tt.port, err)
			}
			if got != tt.want {
				t.Fatalf("Port(%q) = %d, want %d", tt.port, got, tt.want)
			}
		})
	}
}

func TestPortIgnoresOtherVariables(t *testing.T) {
	getenv := func(key string) string {
		if key == "PORT" {
			return "7070"
		}
		return ""
	}
	if got, err := Port(getenv); err != nil || got != 7070 {
		t.Fatalf("Port() = %d, %v; want 7070, nil", got, err)
	}
}
