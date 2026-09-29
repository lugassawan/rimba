package observability

import "testing"

func TestShareSinkForwardsWritesButNotClose(t *testing.T) {
	inner := &fakeSink{}
	shared := ShareSink(inner)

	if err := shared.WriteLog("l"); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	if err := shared.WriteMetric("m"); err != nil {
		t.Fatalf("WriteMetric: %v", err)
	}
	if err := shared.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if len(inner.logs) != 1 || len(inner.metrics) != 1 {
		t.Errorf("writes not forwarded: logs=%v metrics=%v", inner.logs, inner.metrics)
	}
	if inner.closed {
		t.Error("Close reached the inner sink; the owner alone should close it")
	}
}
