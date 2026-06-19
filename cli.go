package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/binRick/rbstat/internal/collect"
	"github.com/binRick/rbstat/internal/collectors"
)

const version = "0.1.0"

// action is what main should do after parsing: run, or print help/version/list.
type action int

const (
	actionRun action = iota
	actionHelp
	actionVersion
	actionList
)

type config struct {
	cols         []collect.Collector
	delay        time.Duration
	count        int  // 0 == run forever
	nocolor      bool
	noheaders    bool
	usingDefault bool // true when no stats were chosen and -cdngy was applied
}

// defaultSet is the dstat default when no stat flags are given (-cdngy).
var defaultSet = []string{"cpu", "disk", "net", "page", "sys"}

// shortFlags maps clustered short flags to collector names.
var shortFlags = map[rune]string{
	'c': "cpu", 'd': "disk", 'n': "net", 'g': "page", 'y': "sys",
	'm': "mem", 's': "swap", 'l': "load", 't': "time", 'r': "io",
}

// longFlags maps --name flags to collector names.
var longFlags = map[string]string{
	"cpu": "cpu", "disk": "disk", "net": "net", "page": "page", "sys": "sys",
	"mem": "mem", "swap": "swap", "load": "load", "time": "time", "io": "io",
}

// allNames is the order in which collectors are listed by --list.
var allNames = []string{"cpu", "disk", "net", "page", "sys", "mem", "swap", "load", "time", "io"}

func newByName(name string) collect.Collector {
	switch name {
	case "cpu":
		return collectors.NewCPU()
	case "disk":
		return collectors.NewDisk()
	case "net":
		return collectors.NewNet()
	case "page":
		return collectors.NewPage()
	case "sys":
		return collectors.NewSys()
	case "mem":
		return collectors.NewMem()
	case "swap":
		return collectors.NewSwap()
	case "load":
		return collectors.NewLoad()
	case "time":
		return collectors.NewTime()
	case "io":
		return collectors.NewIO()
	}
	return nil
}

// parseArgs parses dstat-style arguments: clustered short flags (-cdn), long
// flags (--cpu), behavior flags, and positional [delay [count]]. It has no I/O
// side effects; main handles the returned action and the usingDefault notice.
func parseArgs(args []string) (*config, action, error) {
	cfg := &config{delay: time.Second}
	var names []string
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	var positionals []string
	onlyPos := false
	for _, a := range args {
		switch {
		case onlyPos:
			positionals = append(positionals, a)
		case a == "--":
			onlyPos = true
		case strings.HasPrefix(a, "--"):
			name := a[2:]
			switch name {
			case "version":
				return nil, actionVersion, nil
			case "help":
				return nil, actionHelp, nil
			case "list":
				return nil, actionList, nil
			case "nocolor":
				cfg.nocolor = true
			case "noheaders":
				cfg.noheaders = true
			case "noupdate":
				// scrolling output is the only mode; accept as a no-op.
			case "all":
				for _, n := range defaultSet {
					add(n)
				}
			default:
				if n, ok := longFlags[name]; ok {
					add(n)
				} else {
					return nil, actionRun, fmt.Errorf("unknown option: --%s", name)
				}
			}
		case len(a) > 1 && a[0] == '-' && !isNumber(a[1:]):
			for _, ch := range a[1:] {
				switch {
				case ch == 'a':
					for _, n := range defaultSet {
						add(n)
					}
				case ch == 'h':
					return nil, actionHelp, nil
				default:
					if n, ok := shortFlags[ch]; ok {
						add(n)
					} else {
						return nil, actionRun, fmt.Errorf("unknown flag: -%c", ch)
					}
				}
			}
		default:
			positionals = append(positionals, a)
		}
	}

	if len(positionals) >= 1 {
		f, err := strconv.ParseFloat(positionals[0], 64)
		if err != nil || f <= 0 {
			return nil, actionRun, fmt.Errorf("invalid delay: %s", positionals[0])
		}
		cfg.delay = time.Duration(f * float64(time.Second))
	}
	if len(positionals) >= 2 {
		c, err := strconv.Atoi(positionals[1])
		if err != nil || c < 0 {
			return nil, actionRun, fmt.Errorf("invalid count: %s", positionals[1])
		}
		cfg.count = c
	}
	if len(positionals) > 2 {
		return nil, actionRun, fmt.Errorf("too many arguments: %s", strings.Join(positionals[2:], " "))
	}

	// dstat treats the time plugin as a leading column, not a "stat". When only
	// time (or nothing) is selected, append the -cdngy default set, keeping the
	// time column first.
	if onlyTime(names) {
		cfg.usingDefault = true
		names = append(names, defaultSet...)
	}
	for _, n := range names {
		cfg.cols = append(cfg.cols, newByName(n))
	}
	return cfg, actionRun, nil
}

// onlyTime reports whether the selection contains no real stat (empty, or only
// the time plugin), in which case the default set must be added.
func onlyTime(names []string) bool {
	for _, n := range names {
		if n != "time" {
			return false
		}
	}
	return true
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func printHelp() {
	fmt.Print(`Usage: rbstat [-cdngymslrt] [options] [delay [count]]

A Go clone of the classic Linux dstat resource monitor.

Stat flags (default: -cdngy):
  -c, --cpu     CPU usage (usr/sys/idl/wai/stl)
  -d, --disk    disk throughput (read/writ)
  -n, --net     network throughput (recv/send)
  -g, --page    paging: pages swapped in/out (in/out)
  -y, --sys     system: interrupts and context switches (int/csw)
  -m, --mem     memory usage (used/free/buf/cach)
  -s, --swap    swap usage (used/free)
  -l, --load    load average (1m/5m/15m)
  -t, --time    current time
  -r, --io      disk I/O requests (read/writ)
  -a, --all     equivalent to -cdngy

Options:
      --nocolor    disable ANSI colors
      --noheaders  do not repeat the header
      --noupdate   scrolling output (default; accepted for compatibility)
      --list       list available stats and exit
      --version    print version and exit
  -h, --help       show this help and exit

Arguments:
  delay   seconds between updates (default 1)
  count   number of updates, 0 for unlimited (default 0)
`)
}
