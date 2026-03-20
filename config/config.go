package config

type RunConfig struct {
	Env               string
	Proxy             string
	Out               string
	Interval          string
	Only              []string
	StopOnFailure     bool
	StopOnError       bool
	ParallelExecution bool
	BatchSize         int
	RequestTimeout    int
	TotalTimeout      int
}

type LintConfig struct {
	Env string
}
