package observability

import "context"

type ctxKey struct{}

// WithRecorder attaches rec to ctx, mirroring config.WithConfig/FromContext.
func WithRecorder(ctx context.Context, rec *Recorder) context.Context {
	return context.WithValue(ctx, ctxKey{}, rec)
}

// FromContext returns the Recorder attached to ctx, or nil if absent — nil is
// always safe to call every Recorder method on (see recorder.go).
func FromContext(ctx context.Context) *Recorder {
	rec, _ := ctx.Value(ctxKey{}).(*Recorder)
	return rec
}
