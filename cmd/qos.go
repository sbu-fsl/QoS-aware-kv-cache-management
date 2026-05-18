package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sbu-fsl/qos-aware-restoration/internal/cache"
	"github.com/sbu-fsl/qos-aware-restoration/internal/config"
	"github.com/sbu-fsl/qos-aware-restoration/internal/report"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type QoSCMD struct {
	configPath *string
	dataDir    *string
}

func (c *QoSCMD) Command() *cobra.Command {
	instance := &cobra.Command{
		Use:   "qos",
		Short: fmt.Sprintf("QoS-aware KV Cache Management (CLI v%s)", version),
		Long:  "Run the QoS-aware KV cache management simulator with a terminal interface for testing and demonstration.",
		Run: func(cmd *cobra.Command, args []string) {
			c.main()
		},
	}

	// register flags for the main function
	c.configPath = instance.Flags().String("config", defaultConfigPath, "path to config file")
	c.dataDir = instance.Flags().String("data", defaultDataDir, "directory for tier cache files")

	return instance
}

func (c *QoSCMD) main() {
	// load configs
	cfg, err := config.Load(*c.configPath)
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
	if err := os.MkdirAll(*c.dataDir, 0755); err != nil {
		log.Fatalf("[Err] failed creating data directory: %v\n", err)
	}

	// convert data dir to an absolute path
	absData, err := filepath.Abs(*c.dataDir)
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

	// create a scanner instance
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

			fmt.Print(report.Store(results, true))
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

			fmt.Print(report.Restore(results, overall, true))
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

			fmt.Print(report.Restore(results, overall, true))
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
