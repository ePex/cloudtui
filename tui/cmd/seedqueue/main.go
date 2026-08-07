// Command seedqueue sends sample JSON messages to a queue via the active
// connection's Jolokia backend, for populating a broker with test data
// during local development.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue/jolokia"
	"github.com/ePex/cloudtui/tui/internal/seed"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <queue-name> <count>\n\n"+
			"Sends <count> sample JSON messages to <queue-name> via the active\n"+
			"connection's Jolokia backend (config.yaml).\n", os.Args[0])
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	queueName := flag.Arg(0)
	count, err := strconv.Atoi(flag.Arg(1))
	if err != nil || count < 1 {
		fmt.Fprintf(os.Stderr, "count must be a positive integer, got %q\n", flag.Arg(1))
		os.Exit(2)
	}

	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loading config: %v\n", err)
		os.Exit(1)
	}
	conn := cfg.ActiveConn()
	if conn.Backend != "" && conn.Backend != "jolokia" {
		fmt.Fprintf(os.Stderr, "active connection %q uses backend %q; seedqueue only supports jolokia\n", conn.Name, conn.Backend)
		os.Exit(1)
	}

	client := jolokia.NewClient(conn.Queue)
	err = seed.Run(context.Background(), client, queueName, count, func(sent, total int) {
		fmt.Printf("sent %d/%d to %q\n", sent, total, queueName)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
