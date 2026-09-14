package golib

import "testing"

func TestConfigResolve(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		want    Config
		wantErr bool
	}{
		{
			name:   "zero value gets defaults",
			config: Config{},
			want:   Config{Title: "GoLib", Width: 1280, Height: 720},
		},
		{
			name:   "set fields are kept",
			config: Config{Title: "Pong", Width: 800, Height: 600},
			want:   Config{Title: "Pong", Width: 800, Height: 600},
		},
		{
			name:    "negative width is rejected",
			config:  Config{Width: -1},
			wantErr: true,
		},
		{
			name:    "negative height is rejected",
			config:  Config{Height: -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.resolve()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolve() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRunRejectsNilGame(t *testing.T) {
	if err := Run(nil, Config{}); err == nil {
		t.Fatal("Run(nil, Config{}) returned no error")
	}
}
