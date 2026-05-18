package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sbu-fsl/qos-aware-restoration/internal/cache"
	"github.com/sbu-fsl/qos-aware-restoration/internal/config"
	"github.com/sbu-fsl/qos-aware-restoration/internal/report"
	"github.com/spf13/cobra"

	"gopkg.in/yaml.v3"
)

type AutoRunCMD struct {
	configPath *string
	dataDir    *string
	cmdPath    *string
	outPath    *string
	verbose    *bool
}

func (c *AutoRunCMD) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "autorun",
		Short: "Automated QoS-aware KV Cache Management (CLI)",
		Long:  "Run a sequence of cache operations defined in a cmd.yaml file for testing and demonstration.",
		Run: func(cmd *cobra.Command, args []string) {
			// register flags for the main function
			c.configPath = cmd.Flags().String("config", defaultConfigPath, "path to config file")
			c.dataDir = cmd.Flags().String("data", defaultDataDir, "directory for tier cache files")
			c.cmdPath = cmd.Flags().String("cmd", "cmd.yaml", "path to operations file")
			c.outPath = cmd.Flags().String("out", "out.txt", "path to output report file")
			c.verbose = cmd.Flags().Bool("verbose", false, "enable verbose logging")

			c.main()
		},
	}
}

// Operation represents a single entry in cmd.yaml.
type Operation struct {
	Op     string `yaml:"op"`
	Blocks string `yaml:"blocks"`
}

// CmdFile is the top-level structure of cmd.yaml.
type CmdFile struct {
	Operations []Operation `yaml:"operations"`
}

func (c *AutoRunCMD) main() {
	// load configs
	cfg, err := config.Load(*c.configPath)
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
	if err := os.MkdirAll(*c.dataDir, 0755); err != nil {
		log.Fatalf("[Err] failed creating data directory: %v\n", err)
	}

	// convert data dir to an absolute path
	absData, err := filepath.Abs(*c.dataDir)
	if err != nil {
		log.Fatalf("[Err] failed resolving data path: %v\n", err)
	}

	// create cache engine
	engine, err := cache.NewEngine(cfg, absData)
	if err != nil {
		log.Fatalf("[Err] failed creating engine: %v\n", err)
	}

	// load the cmd config file
	ops, err := loadCmdFile(*c.cmdPath)
	if err != nil {
		log.Fatalf("[Err] failed loading cmd file: %v\n", err)
	}

	// create logs file
	outFile, err := os.Create(*c.outPath)
	if err != nil {
		log.Fatalf("[Err] failed creating output file: %v\n", err)
	}
	defer outFile.Close()

	// loop over operations and execute
	for i, op := range ops {
		ids, err := parseIDs(op.Blocks)
		if err != nil && op.Op != "purge" { // purge doesn't require blocks
			log.Fatalf("[Err] operation %d: invalid blocks %q: %v\n", i+1, op.Blocks, err)
		}

		header := fmt.Sprintf("=== Op %d: %s %s ===\n", i+1, strings.ToUpper(op.Op), fmtList(ids))
		fmt.Fprint(outFile, header)

		switch strings.ToLower(op.Op) {
		case "store":
			results := engine.Store(ids)
			text := report.Store(results, *c.verbose)
			fmt.Fprint(outFile, text)

			// speed line to console only
			fmt.Printf("[Op %d] store %v — %d block(s) stored\n", i+1, fmtList(ids), len(ids))

		case "restore":
			results, overall := engine.RestoreAuto(ids)
			text := report.Restore(results, overall, *c.verbose)
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

	fmt.Printf("\nFull report written to %s\n", *c.outPath)
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
