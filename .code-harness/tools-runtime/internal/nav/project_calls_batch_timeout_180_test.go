package nav

import (
	"context"
	"errors"
	"testing"
	"time"
)

type deadlineBatch180Runner struct{}

func (deadlineBatch180Runner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func Test180BatchDirectCallDeadlineFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	started := time.Now()
	n := Navigator{AstGrepPath: "ast-grep", Runner: deadlineBatch180Runner{}}
	_, err := n.FindDirectMethodCallsBatch180(ctx, "src/main/java")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline must fail closed with DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("deadline propagation was not prompt: %s", elapsed)
	}
}
