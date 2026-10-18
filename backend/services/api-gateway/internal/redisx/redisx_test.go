package redisx

import (
	"strings"
	"testing"
)

func TestClientOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		wantAddr string
		wantPass string
		wantDB   int
		wantTLS  bool
	}{
		{"bare host:port (local dev, Helm)", "localhost:56379", "localhost:56379", "", 0, false},
		{"render internal host:port", "red-abc123:6379", "red-abc123:6379", "", 0, false},
		{"render internal connection string", "redis://red-abc123:6379", "red-abc123:6379", "", 0, false},
		{"url with password and db", "redis://:pw@cache:6380/2", "cache:6380", "pw", 2, false},
		{"tls url", "rediss://default:pw@example.upstash.io:6379", "example.upstash.io:6379", "pw", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := clientOptions(tt.addr)
			if err != nil {
				t.Fatalf("clientOptions(%q): %v", tt.addr, err)
			}
			if opts.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", opts.Addr, tt.wantAddr)
			}
			if opts.Password != tt.wantPass {
				t.Errorf("Password = %q, want %q", opts.Password, tt.wantPass)
			}
			if opts.DB != tt.wantDB {
				t.Errorf("DB = %d, want %d", opts.DB, tt.wantDB)
			}
			if (opts.TLSConfig != nil) != tt.wantTLS {
				t.Errorf("TLS enabled = %v, want %v", opts.TLSConfig != nil, tt.wantTLS)
			}
		})
	}
}

func TestClientOptionsRejectsBadURLWithoutLeakingIt(t *testing.T) {
	t.Parallel()

	_, err := clientOptions("redis://user:hunter2@host:6379/notanumber")
	if err == nil {
		t.Fatal("clientOptions accepted an invalid URL")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("error leaks the password: %v", err)
	}
}
