package main

import (
	"github.com/sbu-fsl/qos-aware-restoration/cmd"

	"github.com/spf13/cobra"
)

func main() {
	// build a root command
	root := cobra.Command{
		Short: "QoS-aware KV Cache Management Simulator",
	}

	// add subcommands
	root.AddCommand(
		(&cmd.AutoRunCMD{}).Command(),
		(&cmd.QoSCMD{}).Command(),
	)

	// execute the root command
	if err := root.Execute(); err != nil {
		panic(err)
	}
}
