package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sbu-fsl/qos-aware-restoration/internal/cache"
	"github.com/sbu-fsl/qos-aware-restoration/internal/config"
	"github.com/sbu-fsl/qos-aware-restoration/internal/report"

	"gopkg.in/yaml.v3"
)

// Operation represents a single entry in cmd.yaml.
type Operation struct {
	Op     string `yaml:"op"`
	Blocks string `yaml:"blocks"`
}

// CmdFile is the top-level structure of cmd.yaml.
type CmdFile struct {
	Operations []Operation `yaml:"operations"`
}

func main() {
	// command-line flags
	var (
		flagCfg  = flag.String("config", "config.yaml", "path to config file")
		flagData = flag.String("data", "data", "directory for tier cache files")
		flagCmd  = flag.String("cmd", "cmd.yaml", "path to operations file")
		flagOut  = flag.String("out", "out.txt", "path to output report file")
	)

	// parse flags
	flag.Parse()

	// load configs
	cfg, err := config.Load(*flagCfg)
	if err != nil {
		log.Fatalf("[Err] failed loading config: %v\n", err)
	}

	// print config summary as JSON for easy parsing
	cfgJSON, err := yaml.Marshal(cfg)
	if err != nil {
		log.Fatalf("[Err] failed marshaling config to JSON: %v\n", err)
	}
	fmt.Printf("Loaded config:\n%s\n", string(cfgJSON))

	// ensure data directory exists
	if err := os.MkdirAll(*flagData, 0755); err != nil {
		log.Fatalf("[Err] failed creating data directory: %v\n", err)
	}

	// convert data dir to an absolute path
	absData, err := filepath.Abs(*flagData)
	if err != nil {
		log.Fatalf("[Err] failed resolving data path: %v\n", err)
	}

	// create cache engine
	engine, err := cache.NewEngine(cfg, absData)
	if err != nil {
		log.Fatalf("[Err] failed creating engine: %v\n", err)
	}

	// load the cmd config file
	ops, err := loadCmdFile(*flagCmd)
	if err != nil {
		log.Fatalf("[Err] failed loading cmd file: %v\n", err)
	}

	// create logs file
	outFile, err := os.Create(*flagOut)
	if err != nil {
		log.Fatalf("[Err] failed creating output file: %v\n", err)
	}
	defer outFile.Close()

	// loop over operations and execute
	for i, op := range ops {
		ids, err := parseIDs(op.Blocks)
		if err != nil {
			log.Fatalf("[Err] operation %d: invalid blocks %q: %v\n", i+1, op.Blocks, err)
		}

		header := fmt.Sprintf("=== Op %d: %s %s ===\n", i+1, strings.ToUpper(op.Op), fmtList(ids))
		fmt.Fprint(outFile, header)

		switch strings.ToLower(op.Op) {
		case "store":
			results := engine.Store(ids)
			text := report.Store(results)
			fmt.Fprint(outFile, text)

			// speed line to console only
			fmt.Printf("[Op %d] store %v — %d block(s) stored\n", i+1, fmtList(ids), len(ids))

		case "restore":
			results, overall := engine.RestoreAuto(ids)
			text := report.Restore(results, overall)
			fmt.Fprint(outFile, text)

			// speed line to console only
			speed := computeSpeed(ids, overall)

			fmt.Printf("[Op %d] restore %v — %.2f blocks/s  (overall: %s)\n", i+1, fmtList(ids), speed, fmtDur(overall))

		case "purge":
			if err := engine.Purge(); err != nil {
				log.Fatalf("[Err] operation %d: purge failed: %v\n", i+1, err)
			}
			fmt.Fprintf(outFile, "Purged blocks.\n")

			// log to console as well
			fmt.Printf("[Op %d] purge — all blocks purged\n", i+1)

		default:
			log.Fatalf("[Err] operation %d: unknown op %q (supported: store, restore)\n", i+1, op.Op)
		}

		fmt.Fprintln(outFile)
	}

	fmt.Printf("\nFull report written to %s\n", *flagOut)
}

// loadCmdFile reads the operations from a `cmd.yaml` format file.
func loadCmdFile(path string) ([]Operation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cf CmdFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}

	return cf.Operations, nil
}

// parseIDs parses a block specification: comma-separated IDs or a range "N-M".
func parseIDs(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty blocks field")
	}

	// range form: "N-M"
	if idx := strings.Index(s, "-"); idx > 0 && !strings.Contains(s, ",") {
		parts := strings.SplitN(s, "-", 2)

		lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid range %q", s)
		}
		if lo > hi {
			return nil, fmt.Errorf("range start %d > end %d", lo, hi)
		}

		ids := make([]int, hi-lo+1)
		for i := range ids {
			ids[i] = lo + i
		}

		return ids, nil
	}

	// comma-separated form
	parts := strings.Split(s, ",")
	ids := make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid block ID %q", p)
		}

		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid block IDs in %q", s)
	}

	return ids, nil
}

// computeSpeed returns `block/sec` based on len and overall time.
func computeSpeed(ids []int, overall time.Duration) float64 {
	if overall == 0 {
		return 0
	}

	return float64(len(ids)) / overall.Seconds()
}

// format duration helper function.
func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}

	return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
}

// format ids helper function.
func fmtList(ids []int) string {
	if len(ids) > 10 {
		return fmt.Sprintf("%v...", ids[:10])
	} else {
		return fmt.Sprintf("%v", ids)
	}
}
