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
//	AGENTLENS_GATEWAY_TENANT_PLANS pares tenant:plan separados por coma
//	AGENTLENS_GATEWAY_DEFAULT_PLAN plan para tenants no listados  (def. free)
//	AGENTLENS_OTLP_ENDPOINT      destino OTLP para auto-observabilidad (opcional)
package main

import (
	"context"
	"log"
	"net"
	"os"
	"strings"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/forward"
	"github.com/bysempet/agentlens/ingestion/internal/gateway"
	"github.com/bysempet/agentlens/ingestion/internal/plan"
	"github.com/bysempet/agentlens/ingestion/internal/ratelimit"
	"github.com/bysempet/agentlens/shared/keystore"
	"github.com/bysempet/agentlens/shared/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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

// parseTenantPlans interpreta "acme:growth,globex:free" como mapa tenant -> plan.
func parseTenantPlans(raw string) map[string]plan.Plan {
	out := map[string]plan.Plan{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		tenant, name, ok := strings.Cut(pair, ":")
		if !ok || tenant == "" {
			log.Fatalf("par tenant:plan inválido: %q", pair)
		}
		p, ok := plan.ByName(name)
		if !ok {
			log.Fatalf("plan desconocido %q para el tenant %q", name, tenant)
		}
		out[tenant] = p
	}
	return out
}

func main() {
	listen := getenv("AGENTLENS_GATEWAY_LISTEN", ":4317")
	downstream := getenv("AGENTLENS_GATEWAY_DOWNSTREAM", "localhost:5317")
	certPath := os.Getenv("AGENTLENS_GATEWAY_TLS_CERT")
	keyPath := os.Getenv("AGENTLENS_GATEWAY_TLS_KEY")
	keys, err := keystore.ParseSpec(os.Getenv("AGENTLENS_GATEWAY_API_KEYS"))
	if err != nil {
		log.Fatal(err)
	}
	if len(keys) == 0 {
		log.Fatal("AGENTLENS_GATEWAY_API_KEYS vacío: no hay claves que aceptar")
	}

	defaultPlan := plan.Free
	if name := os.Getenv("AGENTLENS_GATEWAY_DEFAULT_PLAN"); name != "" {
		p, ok := plan.ByName(name)
		if !ok {
			log.Fatalf("AGENTLENS_GATEWAY_DEFAULT_PLAN desconocido: %q", name)
		}
		defaultPlan = p
	}
	registry := plan.NewStaticRegistry(
		parseTenantPlans(os.Getenv("AGENTLENS_GATEWAY_TENANT_PLANS")),
		defaultPlan,
	)
	limiter := ratelimit.New(registry)

	// Auto-observabilidad (E0-T08): el gateway emite sus propias trazas si hay OTLP.
	shutdown, err := observability.Init(context.Background(), "agentlens-gateway")
	if err != nil {
		log.Fatalf("observabilidad: %v", err)
	}
	defer shutdown(context.Background())

	forwarder, err := forward.NewOTLPForwarder(downstream)
	if err != nil {
		log.Fatalf("no se pudo crear el forwarder a %s: %v", downstream, err)
	}
	defer forwarder.Close()

	var opts []grpc.ServerOption
	// Auto-observabilidad: traza cada RPC del gateway.
	opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	// Cadena: primero auth (resuelve el tenant), luego rate limiting por plan.
	opts = append(opts, grpc.ChainUnaryInterceptor(
		auth.UnaryInterceptor(auth.NewStaticKeyStore(keys)),
		ratelimit.UnaryInterceptor(limiter),
	))
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
