package spinner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Options configures spinner behavior.
type Options struct {
	Writer  io.Writer
	NoColor bool
}

// Spinner provides visual feedback during slow operations.
type Spinner struct {
	w        io.Writer
	animated bool

	mu      sync.Mutex
	running bool
	msg     string
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// New creates a Spinner. If the writer is not a TTY or NoColor is set,
// the spinner falls back to plain line-based output.
func New(opts Options) *Spinner {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	return &Spinner{
		w:        w,
		animated: !opts.NoColor && isTTY(w),
	}
}

// Start begins the spinner with the given message.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.msg = message
		return
	}

	s.msg = message
	s.running = true

	if !s.animated {
		fmt.Fprintln(s.w, message)
		return
	}

	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	go s.animate()
}

// Update changes the spinner message while running.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.msg = message

	if !s.animated {
		fmt.Fprintln(s.w, message)
	}
}

// Stop clears the spinner line and stops the animation.
// Safe to call multiple times and with defer.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false

	if !s.animated {
		s.mu.Unlock()
		return
	}

	close(s.stopCh)
	s.mu.Unlock()

	<-s.doneCh
}

func (s *Spinner) animate() {
	defer close(s.doneCh)

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		s.mu.Lock()
		msg := s.msg
		s.mu.Unlock()

		fmt.Fprintf(s.w, "\r%s %s", frames[i%len(frames)], msg)

		select {
		case <-s.stopCh:
			// Clear the spinner line
			s.clearLine()
			return
		case <-ticker.C:
			i++
		}
	}
}

func (s *Spinner) clearLine() {
	fmt.Fprintf(s.w, "\r\033[K")
}

// isTTY reports whether w is a terminal.
func isTTY(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
