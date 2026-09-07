package nav

import (
	"context"
	"testing"
)

type countingEntrypointRunner164 struct{ calls int }

func (r *countingEntrypointRunner164) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	r.calls++
	return []byte(
		"{\"file\":\"src/main/java/acme/AController.java\",\"text\":\"@RestController\\nclass AController { }\",\"range\":{\"start\":{\"line\":0,\"column\":0},\"end\":{\"line\":20,\"column\":1}}}\n" +
			"{\"file\":\"src/main/java/acme/AController.java\",\"text\":\"@GetMapping\\npublic String get() { return \\\"ok\\\"; }\",\"range\":{\"start\":{\"line\":5,\"column\":2},\"end\":{\"line\":8,\"column\":3}}}\n",
	), nil
}

func Test164Task1BOldEntrypointScannerDoesNotAmplifyAstProcesses(t *testing.T) {
	runner := &countingEntrypointRunner164{}
	n := Navigator{AstGrepPath: "ast-grep.exe", Runner: runner}

	got, err := n.FindControllerEndpoints(context.Background(), "src/main/java/acme/AController.java")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "AController.get" {
		t.Fatalf("unexpected endpoint result: %+v", got)
	}
	if runner.calls > 2 {
		t.Fatalf("single-side entrypoint scan started %d ast-grep processes; batch target is <=2", runner.calls)
	}
}
