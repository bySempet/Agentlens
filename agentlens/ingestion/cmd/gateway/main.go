// Command gateway es el Ingestion Gateway de AgentLens (E2-T05): recibe OTLP/gRPC
// del SDK sobre TLS, autentica por API key, sella el tenant y reenvía al
// Collector downstream.
//
// Configuración por entorno:
//
//	AGENTLENS_GATEWAY_LISTEN     dirección de escucha           (def. :4317)
//	AGENTLENS_GATEWAY_DOWNSTREAM endpoint OTLP del Collector     (def. localhost:5317)
//	AGENTLENS_GATEWAY_TLS_CERT   ruta al certificado TLS         (opcional)
//	AGENTLENS_GATEWAY_TLS_KEY    ruta a la clave privada TLS     (opcional)
//	AGENTLENS_GATEWAY_API_KEYS   pares key:tenant separados por coma
package main

import (
	"log"
	"net"
	"os"
	"strings"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/forward"
	"github.com/bysempet/agentlens/ingestion/internal/gateway"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	listen := getenv("AGENTLENS_GATEWAY_LISTEN", ":4317")
	downstream := getenv("AGENTLENS_GATEWAY_DOWNSTREAM", "localhost:5317")
	certPath := os.Getenv("AGENTLENS_GATEWAY_TLS_CERT")
	keyPath := os.Getenv("AGENTLENS_GATEWAY_TLS_KEY")
	keys := parseAPIKeys(os.Getenv("AGENTLENS_GATEWAY_API_KEYS"))
	if len(keys) == 0 {
		log.Fatal("AGENTLENS_GATEWAY_API_KEYS vacío: no hay claves que aceptar")
	}

	forwarder, err := forward.NewOTLPForwarder(downstream)
	if err != nil {
		log.Fatalf("no se pudo crear el forwarder a %s: %v", downstream, err)
	}
	defer forwarder.Close()

	var opts []grpc.ServerOption
	opts = append(opts, grpc.UnaryInterceptor(auth.UnaryInterceptor(auth.NewStaticKeyStore(keys))))
	if certPath != "" && keyPath != "" {
		creds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
		if err != nil {
			log.Fatalf("no se pudieron cargar las credenciales TLS: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
		log.Printf("TLS habilitado (cert=%s)", certPath)
	} else {
		log.Print("AVISO: TLS deshabilitado (sin cert/key); usar solo en desarrollo")
	}

	srv := grpc.NewServer(opts...)
	coltracepb.RegisterTraceServiceServer(srv, gateway.NewTraceServer(forwarder))

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", listen, err)
	}
	log.Printf("Ingestion Gateway escuchando en %s -> downstream %s", listen, downstream)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("servidor detenido: %v", err)
	}
}
