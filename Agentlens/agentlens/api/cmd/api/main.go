// Command api es la API hot path de lectura de trazas de AgentLens (E3-T01):
// REST sobre ClickHouse, con auth por API key y datos acotados al tenant.
//
// Configuración por entorno:
//
//	AGENTLENS_API_LISTEN               dirección de escucha HTTP  (def. :8080)
//	AGENTLENS_API_CLICKHOUSE           endpoint HTTP de ClickHouse (def. http://localhost:8123)
//	AGENTLENS_API_CLICKHOUSE_USER      usuario de ClickHouse       (def. agentlens)
//	AGENTLENS_API_CLICKHOUSE_PASSWORD  contraseña de ClickHouse    (def. vacía; el
//	                                   docker-compose local usa "agentlens")
//	AGENTLENS_API_KEYS                 entradas key:tenant[:plan] separadas por coma
//	                                   (mismo formato que el gateway; el plan se ignora aquí)
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bysempet/agentlens/api/internal/chstore"
	"github.com/bysempet/agentlens/api/internal/httpapi"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseAPIKeys interpreta "key:tenant[:plan]" (el formato del gateway) como
// mapa key -> tenant. El plan no aplica a la lectura y se descarta.
func parseAPIKeys(raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
			log.Fatalf("par API key inválido (esperado key:tenant[:plan]): %q", entry)
		}
		out[parts[0]] = parts[1]
	}
	return out
}

func main() {
	listen := getenv("AGENTLENS_API_LISTEN", ":8080")
	chURL := getenv("AGENTLENS_API_CLICKHOUSE", "http://localhost:8123")
	chUser := getenv("AGENTLENS_API_CLICKHOUSE_USER", "agentlens")
	// Sin getenv: la contraseña vacía es un valor válido (p. ej. el usuario
	// default de un ClickHouse local).
	chPassword := os.Getenv("AGENTLENS_API_CLICKHOUSE_PASSWORD")
	keys := parseAPIKeys(os.Getenv("AGENTLENS_API_KEYS"))
	if len(keys) == 0 {
		log.Fatal("AGENTLENS_API_KEYS vacío: no hay claves que aceptar")
	}

	st := chstore.New(chURL, chUser, chPassword)
	srv := &http.Server{
		Addr:              listen,
		Handler:           httpapi.NewHandler(st, keys),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	log.Printf("Trace API escuchando en %s -> clickhouse %s", listen, chURL)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("servidor detenido: %v", err)
	}
}
