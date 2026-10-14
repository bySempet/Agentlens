package policy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
)

// TestBuildWASMBundle compila la política a un bundle WASM y verifica que el
// tar.gz resultante contiene policy.wasm (criterio E4-T02).
func TestBuildWASMBundle(t *testing.T) {
	var buf bytes.Buffer
	if err := BuildWASMBundle(context.Background(), &buf, "policies", DefaultEntrypoint, DefaultConfig()); err != nil {
		t.Fatalf("BuildWASMBundle: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("el bundle está vacío")
	}

	files := tarGzFiles(t, buf.Bytes())
	if _, ok := files["/policy.wasm"]; !ok {
		t.Fatalf("el bundle no contiene policy.wasm; entradas=%v", keys(files))
	}

	// Sin data.config el WASM permitiría todo: la configuración debe viajar en el bundle.
	var data struct {
		Config struct {
			BlockedTools []string `json:"blocked_tools"`
			Limits       struct {
				MaxOutputTokens int `json:"max_output_tokens"`
			} `json:"limits"`
		} `json:"config"`
	}
	if err := json.Unmarshal(files["/data.json"], &data); err != nil {
		t.Fatalf("data.json: %v", err)
	}
	want := DefaultConfig()
	if len(data.Config.BlockedTools) != len(want.BlockedTools) ||
		data.Config.Limits.MaxOutputTokens != want.MaxOutputTokens {
		t.Fatalf("data.json no refleja DefaultConfig: %+v", data.Config)
	}
}

func tarGzFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()

	files := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		files[h.Name] = body
	}
	return files
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
