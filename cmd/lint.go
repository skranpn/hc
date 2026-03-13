package cmd

import (
	"fmt"
	"os"

	"github.com/skranpn/hc"
	"github.com/skranpn/hc/config"
	"github.com/spf13/cobra"
)

var lintConfig config.LintConfig

var lintCmd = &cobra.Command{
	Use:   "lint [http_files ...]",
	Short: "Static analysis of HTTP files",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		env, err := hc.LoadEnvFile(lintConfig.Env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load env file: %v\n", err)
			return err
		}

		hasError := false
		for _, path := range args {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: failed to open file: %v\n", path, err)
				hasError = true
				continue
			}

			parser := hc.NewParser()
			reqs, err := parser.Parse(f)
			f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: failed to parse: %v\n", path, err)
				hasError = true
				continue
			}

			issues := hc.Lint(reqs, env)
			for _, issue := range issues {
				reqLabel := fmt.Sprintf("#%d", issue.RequestIndex+1)
				if issue.RequestName != "" {
					reqLabel = fmt.Sprintf("#%d (%s)", issue.RequestIndex+1, issue.RequestName)
				}
				fmt.Printf("%s: [%s] request %s: %s\n", path, issue.Severity, reqLabel, issue.Message)

				if issue.Severity == hc.LintError {
					hasError = true
				}
			}
		}

		if hasError {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
	lintCmd.Flags().StringVarP(&lintConfig.Env, "env", "e", "", "Path to env file")
}
