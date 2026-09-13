package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/config"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/internal/engine"
	"github.com/eddywiyatno/tomcat-monitoring/tmctl/pkg/termutil"
)

// ShowStatus retrieves and displays running containers and health status.
func ShowStatus(ctx context.Context, client engine.EngineClient, cfg *config.StackConfig) error {
	termutil.Info("Inspecting platform containers on %s (%s)...", client.GetInfo().EngineType, client.GetInfo().SocketPath)

	containers, err := client.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	knownWorkloads := map[string]string{
		"tomcat-jmx-exporter": "Tomcat JMX Exporter",
		"prometheus":          "Prometheus TSDB",
		"alertmanager":        "Alertmanager",
		"diagnostic-service":  "Tomcat Diagnostic Service",
		"mailpit":             "Mailpit Test Inbox",
		"postfix-relay":       "Postfix Enterprise SMTP Relay",
	}

	headers := []string{"SERVICE NAME", "CONTAINER NAME", "IMAGE", "STATUS", "PORTS"}
	var rows [][]string

	for name, label := range knownWorkloads {
		var found *engine.ContainerSummary
		for _, c := range containers {
			for _, cName := range c.Names {
				cleanName := strings.TrimPrefix(cName, "/")
				if cleanName == name {
					found = &c
					break
				}
			}
			if found != nil {
				break
			}
		}

		if found != nil {
			var ports []string
			for _, p := range found.Ports {
				if p.PublicPort > 0 {
					ports = append(ports, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
				}
			}
			portsStr := strings.Join(ports, ", ")
			if portsStr == "" {
				portsStr = "-"
			}
			rows = append(rows, []string{
				label,
				name,
				found.Image,
				found.Status,
				portsStr,
			})
		} else {
			rows = append(rows, []string{
				label,
				name,
				"-",
				"Not Created / Stopped",
				"-",
			})
		}
	}

	fmt.Println()
	termutil.PrintTable(os.Stdout, headers, rows)
	fmt.Println()
	return nil
}
