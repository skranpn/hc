package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "hc",
	Short:         "hc is a .http file client",
	Long:          `hc is a .http file client.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	defaultCommand := "run"
	var cmdFound bool
	cmd := rootCmd.Commands()

	for _, a := range cmd {
		for _, b := range os.Args[1:] {
			if a.Name() == b {
				cmdFound = true
				break
			}
		}
	}
	if !cmdFound {
		args := append([]string{defaultCommand}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
