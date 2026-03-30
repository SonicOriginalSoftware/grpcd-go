package client

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// getPollingInterval returns the grpcd polling interval from env or uses a default
func getPollingInterval(log *slog.Logger) time.Duration {
	intervalStr := os.Getenv("GRPCD_REFRESH_INTERVAL_MINUTES")
	def := 5 * time.Minute
	if intervalStr == "" {
		log.Info("Using default refresh interval value", slog.String("value", def.String()))
		return def
	}

	intervalSec, err := strconv.Atoi(intervalStr)
	if err != nil || intervalSec <= 0 {
		log.Error("Invalid refresh interval value", slog.String("value", intervalStr))
		return def
	}

	return time.Duration(intervalSec) * time.Second
}

// Run the poller
func (c *Client) Run(ctx context.Context) {
	c.log.InfoContext(ctx, "Starting poller")
	defer c.deregister(ctx)

	for {
		// Register at the start of each iteration (including first)
		c.register(ctx)

		// Hot-refresh interval from env on each iteration
		interval := getPollingInterval(c.log)

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			// Continue to next iteration
		}
	}
}
