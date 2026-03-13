package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/skranpn/hc"
	"github.com/skranpn/hc/config"
	"golang.org/x/term"

	"github.com/spf13/cobra"
)

var runConfig config.RunConfig

var runCmd = &cobra.Command{
	Use:   "run [http_files ...]",
	Short: "Run API tests defined in HTTP files",
	Long:  `Run API tests defined in HTTP files.`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Create context with timeout if specified
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if runConfig.TotalTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(runConfig.TotalTimeout)*time.Second)
			defer cancel()
		}

		httpFile, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "httpfile not found: %v\n", err)
			return err
		}
		defer httpFile.Close()

		env, err := hc.LoadEnvFile(runConfig.Env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load env file: %v\n", err)
			return err
		}

		// Setup variable manager
		vm := hc.NewVariableManager(env)

		repoter := hc.NewReporter(runConfig.Out)

		// Setup client
		httpClient := http.DefaultClient
		if runConfig.Proxy != "" {
			url, err := url.ParseRequestURI(runConfig.Proxy)
			if err != nil {
				fmt.Fprintf(os.Stderr, "proxy url is invalid, %v\n", err)
				return err
			}
			httpClient = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(url),
				},
			}
		}

		client := hc.NewHttpClient(httpClient)
		pauseCtl := hc.NewPauseController()
		runner := hc.NewRunner(client, vm, repoter, pauseCtl,
			hc.SetStopOnFailure(runConfig.StopOnFailure),
			hc.SetStopOnError(runConfig.StopOnError),
			hc.SetRequestTimeout(runConfig.RequestTimeout),
			hc.SetInterval(runConfig.Interval),
		)

		// Parse HTTP file
		parser := hc.NewParser()
		reqs, err := parser.Parse(httpFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse HTTP file: %v\n", err)
			return err
		}
		if len(reqs) == 0 {
			fmt.Fprint(os.Stderr, "no requests found in HTTP file")
			return nil
		}

		// 標準入力をRawモードに設定
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to set terminal raw mode, %v", err)
		}
		// 終了時に元のモードに戻す（重要！）
		defer func() {
			if oldState != nil {
				term.Restore(int(os.Stdin.Fd()), oldState)
			}
		}()

		go func() {
			b := make([]byte, 1)
			for {
				os.Stdin.Read(b)
				switch b[0] {
				case ' ':
					pauseCtl.Toggle()
				case 0x03: // Ctrl+C
					cancel()
				}
			}
		}()

		// Execute requests (sequential or parallel)
		batchRunner := hc.NewBatch(runner, runConfig.ParallelExecution, runConfig.BatchSize)
		if err := batchRunner.Run(ctx, reqs); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runConfig.Env, "env", "e", "", "Path to env file")
	runCmd.Flags().StringVarP(&runConfig.Proxy, "proxy", "p", "", "Proxy URL")
	runCmd.Flags().StringVarP(&runConfig.Out, "out", "o", "out", "Output directory for results")
	runCmd.Flags().IntVarP(&runConfig.Interval, "interval", "i", 1000, "request interval, defaults to 1000 ms")
	runCmd.Flags().BoolVar(&runConfig.StopOnFailure, "stop-on-failure", false, "Stop execution on assertion failure")
	runCmd.Flags().BoolVar(&runConfig.StopOnError, "stop-on-error", false, "Stop execution on any error")
	runCmd.Flags().BoolVar(&runConfig.ParallelExecution, "parallel", false, "Enable parallel execution of requests")
	runCmd.Flags().IntVarP(&runConfig.BatchSize, "jobs", "j", 4, "Number of requests to run simultaneously")
	runCmd.Flags().IntVar(&runConfig.RequestTimeout, "request-timeout", 30, "Request timeout in seconds (0 = no timeout)")
	runCmd.Flags().IntVar(&runConfig.TotalTimeout, "total-timeout", 0, "Total timeout in seconds (0 = no timeout)")
}
