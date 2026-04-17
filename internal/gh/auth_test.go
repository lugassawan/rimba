package gh

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCheckAuthGhMissing(t *testing.T) {
	t.Setenv("PATH", "")

	err := CheckAuth(context.Background(), &mockRunner{
		run: func(context.Context, ...string) ([]byte, error) {
			t.Fatal("runner should not be called when gh is missing")
			return nil, nil
		},
	})

	assertContains(t, err, "gh CLI not found on PATH")
	assertContains(t, err, "To fix: install from https://cli.github.com and run: gh auth login")
}

func TestCheckAuthNotAuthenticated(t *testing.T) {
	withFakeGhOnPath(t)

	runner := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("You are not logged into any GitHub hosts."), errors.New("exit status 1")
		},
	}

	err := CheckAuth(context.Background(), runner)
	assertContains(t, err, "gh is not authenticated")
	assertContains(t, err, "To fix: run: gh auth login")
}

func TestCheckAuthOK(t *testing.T) {
	withFakeGhOnPath(t)

	var captured []string
	runner := &mockRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			captured = args
			return []byte("Logged in to github.com as someone"), nil
		},
	}

	if err := CheckAuth(context.Background(), runner); err != nil {
		t.Fatalf("CheckAuth() = %v, want nil", err)
	}
	if !reflect.DeepEqual(captured, []string{"auth", "status"}) {
		t.Errorf("captured args = %v, want [auth status]", captured)
	}
}
