// Command buildbundle compila la política Rego a un bundle OPA con WebAssembly
// (E4-T02), listo para enforcement en el borde (SDK/gateway evalúan el WASM
// local). Se invoca en CI; la salida es un bundle.tar.gz con policy.wasm.
//
//	go run ./cmd/buildbundle -out build/bundle.tar.gz
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/bysempet/agentlens/policy-engine/policy"
)

func main() {
	out := flag.String("out", "build/bundle.tar.gz", "ruta del bundle de salida")
	src := flag.String("src", "policy/policies", "directorio con las políticas Rego")
	entrypoint := flag.String("entrypoint", policy.DefaultEntrypoint, "entrypoint (path data.*)")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("creando directorio de salida: %v", err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("creando %s: %v", *out, err)
	}
	defer f.Close()

	if err := policy.BuildWASMBundle(context.Background(), f, *src, *entrypoint); err != nil {
		log.Fatalf("%v", err)
	}
	log.Printf("bundle WASM escrito en %s (entrypoint=%s)", *out, *entrypoint)
}
