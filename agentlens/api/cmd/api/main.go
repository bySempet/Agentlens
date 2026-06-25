// Command api es la API hot path de lectura de trazas de AgentLens (E3-T01):
// listado y detalle de trazas sobre ClickHouse, con paginación.
//
// Configuración por entorno:
//
//	AGENTLENS_API_LISTEN     dirección de escucha HTTP        (def. :8080)
//	AGENTLENS_CLICKHOUSE_ADDR endpoint nativo de ClickHouse   (def. localhost:9000)
//	AGENTLENS_CLICKHOUSE_DB   base de datos                   (def. agentlens)
//	AGENTLENS_CLICKHOUSE_USER usuario                         (def. agentlens)
//	AGENTLENS_CLICKHOUSE_PASS contraseña                      (def. agentlens)
//	AGENTLENS_API_KEYS        pares key:tenant separados por coma (auth)
package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/bysempet/agentlens/api/internal/auth"
	"github.com/bysempet/agentlens/api/internal/httpapi"
	"github.com/bysempet/agentlens/api/internal/store"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseAPIKeys interpreta "key1:tenantA,key2:tenantB" como mapa key -> tenant.
func parseAPIKeys(raw string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, t, ok := strings.Cut(pair, ":")
		if !ok || k == "" || t == "" {
			log.Fatalf("par API key inválido (esperado key:tenant): %q", pair)
		}
		out[k] = t
	}
	return out
}

func main() {
	listen := getenv("AGENTLENS_API_LISTEN", ":8080")
	keys := parseAPIKeys(os.Getenv("AGENTLENS_API_KEYS"))
	if len(keys) == 0 {
		log.Fatal("AGENTLENS_API_KEYS vacío: no hay claves con las que autenticar")
	}

	st, err := store.NewClickHouseStore(
		getenv("AGENTLENS_CLICKHOUSE_ADDR", "localhost:9000"),
		getenv("AGENTLENS_CLICKHOUSE_DB", "agentlens"),
		getenv("AGENTLENS_CLICKHOUSE_USER", "agentlens"),
		getenv("AGENTLENS_CLICKHOUSE_PASS", "agentlens"),
	)
	if err != nil {
		log.Fatalf("no se pudo conectar a ClickHouse: %v", err)
	}
	defer st.Close()

	log.Printf("API hot path escuchando en %s", listen)
	if err := http.ListenAndServe(listen, httpapi.New(st, auth.NewStaticKeyStore(keys))); err != nil {
		log.Fatalf("servidor detenido: %v", err)
	}
}
