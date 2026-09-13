package orchestrator

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/pkg/termutil"
)

// ReadinessProbe checks if a deployed service is accepting traffic and healthy.
type ReadinessProbe struct {
	Timeout  time.Duration
	Interval time.Duration
}

// NewReadinessProbe creates a probe with default or customized timeouts.
func NewReadinessProbe(timeout, interval time.Duration) *ReadinessProbe {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if interval <= 0 {
		interval = 1 * time.Second
	}
	return &ReadinessProbe{
		Timeout:  timeout,
		Interval: interval,
	}
}

// CheckHTTP probes an HTTP/HTTPS endpoint until 200 OK or timeout.
func (p *ReadinessProbe) CheckHTTP(ctx context.Context, targetName, url string) error {
	deadline := time.Now().Add(p.Timeout)
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := httpClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					return nil
				}
			}
		}

		time.Sleep(p.Interval)
	}

	return fmt.Errorf("service %s did not become ready at %s within %v", targetName, url, p.Timeout)
}

// CheckTCP probes a TCP port until connection succeeds or timeout.
func (p *ReadinessProbe) CheckTCP(ctx context.Context, targetName string, host string, port int) error {
	deadline := time.Now().Add(p.Timeout)
	targetAddr := fmt.Sprintf("%s:%d", host, port)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		time.Sleep(p.Interval)
	}

	return fmt.Errorf("service %s port %d unreachable within %v", targetName, port, p.Timeout)
}

// ProbeService automatically checks readiness based on known service patterns.
func (p *ReadinessProbe) ProbeService(ctx context.Context, serviceName string, hostPort int) error {
	switch serviceName {
	case "tomcat-jmx-exporter":
		url := fmt.Sprintf("https://127.0.0.1:%d/metrics", hostPort)
		termutil.Info("Probing Tomcat JMX Exporter HTTPS metrics: %s", url)
		return p.CheckHTTP(ctx, serviceName, url)
	case "prometheus":
		url := fmt.Sprintf("http://127.0.0.1:%d/-/ready", hostPort)
		termutil.Info("Probing Prometheus readiness endpoint: %s", url)
		return p.CheckHTTP(ctx, serviceName, url)
	case "alertmanager":
		url := fmt.Sprintf("http://127.0.0.1:%d/-/ready", hostPort)
		termutil.Info("Probing Alertmanager readiness endpoint: %s", url)
		return p.CheckHTTP(ctx, serviceName, url)
	case "diagnostic-service":
		url := fmt.Sprintf("https://127.0.0.1:%d/health", hostPort)
		termutil.Info("Probing Diagnostic Service HTTPS health: %s", url)
		return p.CheckHTTP(ctx, serviceName, url)
	case "mailpit":
		url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/info", hostPort)
		termutil.Info("Probing Mailpit API endpoint: %s", url)
		return p.CheckHTTP(ctx, serviceName, url)
	case "postfix-relay":
		termutil.Info("Probing Postfix SMTP port: %d", hostPort)
		return p.CheckTCP(ctx, serviceName, "127.0.0.1", hostPort)
	default:
		return p.CheckTCP(ctx, serviceName, "127.0.0.1", hostPort)
	}
}
