package deps

// fakeSink is a tiny in-memory observability.Sink test double, mirroring
// internal/executor/record_test.go's fakeSink. Shared by hooks_test.go and
// manager_test.go.
type fakeSink struct {
	logs    []any
	metrics []any
}

func (f *fakeSink) WriteLog(record any) error {
	f.logs = append(f.logs, record)
	return nil
}

func (f *fakeSink) WriteMetric(record any) error {
	f.metrics = append(f.metrics, record)
	return nil
}

func (f *fakeSink) Close() error {
	return nil
}
