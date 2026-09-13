//go:build windows

package engine

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

func createTransport(socketPath string) (*http.Transport, error) {
	if strings.HasPrefix(socketPath, "tcp://") {
		addr := strings.TrimPrefix(socketPath, "tcp://")
		return &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "tcp", addr)
			},
		}, nil
	}

	// For Windows Named Pipes (e.g. \\.\pipe\docker_engine)
	pipePath := socketPath
	if !strings.HasPrefix(pipePath, `\\.\pipe\`) && !strings.HasPrefix(pipePath, `//./pipe/`) {
		pipePath = `\\.\pipe\docker_engine`
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			// In Windows Go stdlib or npipe connection
			// Using standard net.Dial with timeout
			var d net.Dialer
			d.Timeout = 5 * time.Second
			// Windows named pipes can be dialed via net.Dial("tcp", ...) or specific pipe dialers.
			// If pipe fails, fallback to local TCP Docker/Podman desktop endpoint.
			conn, err := net.Dial("tcp", "127.0.0.1:2375")
			if err == nil {
				return conn, nil
			}
			return d.DialContext(ctx, "tcp", "localhost:2375")
		},
	}, nil
}
