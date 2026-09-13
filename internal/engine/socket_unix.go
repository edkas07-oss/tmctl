//go:build !windows

package engine

import (
	"context"
	"net"
	"net/http"
	"strings"
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

	// Unix Domain Socket
	cleanPath := strings.TrimPrefix(socketPath, "unix://")
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", cleanPath)
		},
	}, nil
}
