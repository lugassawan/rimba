package deps

import "sync"

// fakeSink is a tiny in-memory observability.Sink test double, mirroring
// internal/executor/record_test.go's fakeSink. Shared by hooks_test.go and
// manager_test.go. Guarded by mu since parallel-mode hook tests write to it
// concurrently.
type fakeSink struct {
	mu      sync.Mutex
	logs    []any
	metrics []any
}

func (f *fakeSink) WriteLog(record any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, record)
	return nil
}

func (f *fakeSink) WriteMetric(record any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics = append(f.metrics, record)
	return nil
}

func (f *fakeSink) Close() error {
	return nil
}
