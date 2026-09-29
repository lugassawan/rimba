package cmd

import (
	"fmt"
	"os"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/envvar"
	"github.com/lugassawan/rimba/internal/observability"
)

// openObservabilitySink returns nil when cfg is nil, observability is disabled,
// or opening fails — a broken cache dir must never block a command (see RIMBA_DEBUG).
func openObservabilitySink(cfg *config.Config, repoRoot string) observability.Sink {
	if cfg == nil || !cfg.IsObservabilityEnabled() {
		return nil
	}
	sink, err := observability.NewFileSink(repoRoot, cfg.ObservabilityRetentionDays())
	if err != nil {
		if os.Getenv(envvar.Debug) != "" {
			fmt.Fprintf(os.Stderr, "\n[debug] observability disabled: %v\n", err)
		}
		return nil
	}
	return sink
}
