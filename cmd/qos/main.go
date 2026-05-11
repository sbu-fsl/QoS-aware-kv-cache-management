package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sbu-fsl/qos-aware-restoration/internal/cache"
	"github.com/sbu-fsl/qos-aware-restoration/internal/config"
	"github.com/sbu-fsl/qos-aware-restoration/internal/report"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config.yaml"
	defaultDataDir    = "data"
)

func main() {
	// command-line flags
	var (
		flagCfgPath = flag.String("config", defaultConfigPath, "path to config file")
		flagDataDir = flag.String("data", defaultDataDir, "directory for tier cache files")
	)

	// parse flags
	flag.Parse()

	// load configs
	cfg, err := config.Load(*flagCfgPath)
	if err != nil {
		log.Fatalf("[Err] failed loading configs: %v\n", err)
	}

	// print config summary as JSON for easy parsing
	cfgJSON, err := yaml.Marshal(cfg)
	if err != nil {
		log.Fatalf("[Err] failed marshaling config to JSON: %v\n", err)
	}
	fmt.Printf("Loaded config:\n%s\n", string(cfgJSON))

	// ensure data directory exists
	if err := os.MkdirAll(*flagDataDir, 0755); err != nil {
		log.Fatalf("[Err] failed creating data directory: %v\n", err)
	}

	// convert data dir to an absolute path
	absData, err := filepath.Abs(*flagDataDir)
	if err != nil {
		log.Fatalf("[Err] failed getting absolute path of data directory: %v\n", err)
	}

	// create cache engine
	engine, err := cache.NewEngine(cfg, absData)
	if err != nil {
		log.Fatalf("[Err] failed creating engine: %v\n", err)
	}

	// start CLI
	runCLI(engine)
}

// runCLI opens a terminal interface to get user commands.
func runCLI(engine *cache.Engine) {
	fmt.Println("QoS-aware KV Cache Simulator  —  type 'help' for commands")

	// create a scanner instancr
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// split the line to get the user input
		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToLower(parts[0])

		var args string
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		// run the command
		switch cmd {
		case "help":
			printHelp()
		case "store":
			ids, err := parseIDs(args)
			if err != nil {
				fmt.Println("error:", err)
				continue
			}

			results := engine.Store(ids)

			fmt.Print(report.Store(results))
		case "lookup":
			ids, err := parseIDs(args)
			if err != nil {
				fmt.Println("error:", err)
				continue
			}

			results := engine.Lookup(ids)

			fmt.Print(report.Lookup(results))
		case "restore":
			ids, err := parseIDs(args)
			if err != nil {
				fmt.Println("error:", err)
				continue
			}

			results, overall := engine.RestoreAuto(ids)

			fmt.Print(report.Restore(results, overall))
		case "purge":
			if err := engine.Purge(); err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println("cache purged successfully")
			}
		case "qos-restore":
			ids, err := parseIDs(args)
			if err != nil {
				fmt.Println("error:", err)
				continue
			}

			results, overall := engine.QoSRestore(ids)

			fmt.Print(report.Restore(results, overall))
		case "status":
			fmt.Print(report.Status(engine.Status()))
		case "exit", "quit":
			fmt.Println("bye")
			return
		default:
			fmt.Printf("unknown command %q — type 'help'\n", cmd)
		}
	}
}

// parseIDs accept a blockID list and converts it to a list of integers.
func parseIDs(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("no block IDs provided")
	}

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
		return nil, fmt.Errorf("no valid block IDs")
	}

	return ids, nil
}

// print help function.
func printHelp() {
	fmt.Print(`Commands:
  store   <id,...>   Store blocks across all tiers
  lookup  <id,...>   Check which tiers have each block
  restore     <id,...>   Restore blocks (mode from config: greedy or qos)
  qos-restore <id,...>   Force QoS-aware restore (Ford-Fulkerson) regardless of config
  status             Show tier fill levels
  help               Show this help
  exit               Quit

Examples:
  store 1,2,3,4,5
  lookup 1,3,9
  restore 1,2,3,7,9
`)
}
