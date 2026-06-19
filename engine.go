package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binRick/rbstat/internal/collect"
	"github.com/binRick/rbstat/internal/color"
	"github.com/binRick/rbstat/internal/render"
)

// run drives the sample loop: prime since-boot averages, then sample, render,
// and sleep each interval until the count is reached or interrupted.
func run(cfg *config) error {
	fd := os.Stdout.Fd()
	isTTY := isatty(fd)
	if cfg.nocolor || !isTTY {
		color.Enabled = false
	}

	// Seed rate collectors with the boot time so the first row reads as the
	// average since boot (like classic dstat / vmstat).
	if up, err := collect.ReadUptime(); err == nil {
		boot := time.Now().Add(-time.Duration(up * float64(time.Second)))
		for _, c := range cfg.cols {
			if p, ok := c.(collect.Primer); ok {
				p.Prime(boot)
			}
		}
	}

	r := render.New(cfg.cols, os.Stdout, cfg.noheaders, fd, isTTY)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(cfg.delay)
	defer ticker.Stop()

	for i := 0; ; i++ {
		now := time.Now()
		collect.BeginTick()
		values := make([][]float64, len(cfg.cols))
		for j, c := range cfg.cols {
			v, err := c.Collect(now)
			if err != nil {
				v = nil // render blanks a missing column
			}
			values[j] = v
		}
		r.Tick(values)

		if cfg.count > 0 && i+1 >= cfg.count {
			break
		}
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr)
			return nil
		case <-ticker.C:
		}
	}
	return nil
}
