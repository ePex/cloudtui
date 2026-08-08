// Command devtool provides ad-hoc helpers for live-testing the TUI against
// a real broker: creating/removing disposable queues via JMX (ActiveMQ's
// sendTextMessage requires the destination to already exist), and
// starting/stopping a local mq-proxy instance to test the proxy backend.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/devtool"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  %[1]s add-queue <name>       create a disposable queue via JMX
  %[1]s remove-queue <name>    delete a queue via JMX (drops its messages)
  %[1]s start-proxy            build and start mq-proxy in the background
  %[1]s stop-proxy             stop the mq-proxy instance started above
  %[1]s add-proxy-conn <name> <alias> <url> <username> <password>
                          append a proxy-backend connection to config.yaml
`, os.Args[0])
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx := context.Background()

	switch os.Args[1] {
	case "add-queue", "remove-queue":
		if len(os.Args) != 3 {
			usage()
			os.Exit(2)
		}
		cfg, err := activeQueueConfig()
		if err != nil {
			fail(err)
		}
		if os.Args[1] == "add-queue" {
			err = devtool.AddQueue(ctx, cfg, os.Args[2])
		} else {
			err = devtool.RemoveQueue(ctx, cfg, os.Args[2])
		}
		if err != nil {
			fail(err)
		}
		fmt.Printf("%s %q done\n", os.Args[1], os.Args[2])

	case "start-proxy":
		if err := devtool.StartProxy(ctx, 30*time.Second); err != nil {
			fail(err)
		}
		fmt.Println("mq-proxy is up at http://localhost:8080")

	case "stop-proxy":
		if err := devtool.StopProxy(); err != nil {
			fail(err)
		}
		fmt.Println("mq-proxy stopped")

	case "add-proxy-conn":
		if len(os.Args) != 7 {
			usage()
			os.Exit(2)
		}
		cfg, err := config.LoadDefault()
		if err != nil {
			fail(fmt.Errorf("loading config: %w", err))
		}
		updated, err := devtool.AddProxyConnection(cfg, os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6])
		if err != nil {
			fail(err)
		}
		if err := config.SaveDefault(updated); err != nil {
			fail(fmt.Errorf("saving config: %w", err))
		}
		fmt.Printf("added connection %q (alias %q)\n", os.Args[2], os.Args[3])

	default:
		usage()
		os.Exit(2)
	}
}

// activeQueueConfig returns the Jolokia connection settings to run JMX
// operations against — the active connection if it's a jolokia one, else an
// error, since addQueue/removeQueue only make sense on the ActiveMQ broker.
func activeQueueConfig() (config.QueueConfig, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return config.QueueConfig{}, fmt.Errorf("loading config: %w", err)
	}
	conn := cfg.ActiveConn()
	if conn.Backend != "" && conn.Backend != "jolokia" {
		return config.QueueConfig{}, fmt.Errorf("active connection %q uses backend %q; add-queue/remove-queue need a jolokia connection active", conn.Name, conn.Backend)
	}
	return conn.Queue, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
