package web

import (
	"strings"
	"testing"
)

func TestInjectViteDevHTML(t *testing.T) {
	input := `<html><head><title>x</title></head><body><script type="module" src="/src/main.ts"></script></body></html>`
	out := string(injectViteDevHTML(input))

	if !strings.Contains(out, `src="/__vite/@vite/client"`) {
		t.Fatalf("missing vite client script: %s", out)
	}
	if !strings.Contains(out, `src="/__vite/src/main.ts"`) {
		t.Fatalf("missing vite entry script: %s", out)
	}
	if strings.Contains(out, `src="/src/main.ts"`) {
		t.Fatalf("source entry script should be replaced: %s", out)
	}
}
