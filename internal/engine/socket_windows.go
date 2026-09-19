//go:build windows

package engine

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	winio "github.com/Microsoft/go-winio"
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
			conn, err := winio.DialPipeContext(ctx, pipePath)
			if err == nil {
				return conn, nil
			}

			// Fallback to local TCP Docker daemon if pipe connection fails
			var d net.Dialer
			d.Timeout = 5 * time.Second
			return d.DialContext(ctx, "tcp", "127.0.0.1:2375")
		},
	}, nil
}
