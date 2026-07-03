package policy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"
)

// TestBuildWASMBundle compila la política a un bundle WASM y verifica que el
// tar.gz resultante contiene policy.wasm (criterio E4-T02).
func TestBuildWASMBundle(t *testing.T) {
	var buf bytes.Buffer
	if err := BuildWASMBundle(context.Background(), &buf, "policies", DefaultEntrypoint); err != nil {
		t.Fatalf("BuildWASMBundle: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("el bundle está vacío")
	}

	names := tarGzEntries(t, buf.Bytes())
	if !hasSuffix(names, "policy.wasm") {
		t.Fatalf("el bundle no contiene policy.wasm; entradas=%v", names)
	}
}

func tarGzEntries(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()

	var names []string
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names = append(names, h.Name)
	}
	return names
}

func hasSuffix(names []string, suffix string) bool {
	for _, n := range names {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}
