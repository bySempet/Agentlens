// Command gateway es el Ingestion Gateway de AgentLens (E2-T05, E2-T06): recibe
// OTLP/gRPC del SDK sobre TLS, autentica por API key, aplica el límite de
// ingesta del plan del tenant, sella el tenant y reenvía al Collector.
//
// Configuración por entorno:
//
//	AGENTLENS_GATEWAY_LISTEN      dirección de escucha            (def. :4317)
//	AGENTLENS_GATEWAY_DOWNSTREAM  endpoint OTLP del Collector      (def. localhost:5317)
//	AGENTLENS_GATEWAY_TLS_CERT    ruta al certificado TLS          (opcional)
//	AGENTLENS_GATEWAY_TLS_KEY     ruta a la clave privada TLS      (opcional)
//	AGENTLENS_GATEWAY_API_KEYS    pares key:tenant[:plan] separados por coma
//	                              (plan por defecto: starter)
//	AGENTLENS_GATEWAY_PLAN_LIMITS sobreescribe/añade planes: plan=spans_s[:burst]
//	                              separados por coma; 0 = sin límite
//	                              (ej. "starter=100:200,interno=0")
package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/forward"
	"github.com/bysempet/agentlens/ingestion/internal/gateway"
	"github.com/bysempet/agentlens/ingestion/internal/ratelimit"
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

// parseAPIKeys interpreta "key1:tenantA,key2:tenantB:pro" como mapa
// key -> Tenant. El plan es opcional y por defecto ratelimit.DefaultPlan.
func parseAPIKeys(raw string) map[string]auth.Tenant {
	out := map[string]auth.Tenant{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
			log.Fatalf("par API key inválido (esperado key:tenant[:plan]): %q", entry)
		}
		plan := ratelimit.DefaultPlan
		if len(parts) == 3 {
			if parts[2] == "" {
				log.Fatalf("plan vacío en la entrada %q", entry)
			}
			plan = parts[2]
		}
		out[parts[0]] = auth.Tenant{ID: parts[1], Plan: plan}
	}
	return out
}

// parsePlanLimits interpreta "starter=100:200,interno=0" y lo aplica sobre los
// planes por defecto (sobreescribe los existentes, añade los nuevos). El burst
// es opcional; si falta, se usa 2x el caudal (mismo ratio que los defaults).
func parsePlanLimits(raw string) map[string]ratelimit.Limits {
	plans := ratelimit.DefaultPlans()
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, spec, ok := strings.Cut(entry, "=")
		if !ok || name == "" || spec == "" {
			log.Fatalf("límite de plan inválido (esperado plan=spans_s[:burst]): %q", entry)
		}
		rateStr, burstStr, hasBurst := strings.Cut(spec, ":")
		rate, err := strconv.ParseFloat(rateStr, 64)
		if err != nil {
			log.Fatalf("caudal inválido en %q: %v", entry, err)
		}
		burst := rate * 2
		if hasBurst {
			burst, err = strconv.ParseFloat(burstStr, 64)
			if err != nil {
				log.Fatalf("burst inválido en %q: %v", entry, err)
			}
		}
		plans[name] = ratelimit.Limits{SpansPerSecond: rate, Burst: burst}
	}
	return plans
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
	plans := parsePlanLimits(os.Getenv("AGENTLENS_GATEWAY_PLAN_LIMITS"))

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
	limiter := ratelimit.New(plans)
	coltracepb.RegisterTraceServiceServer(srv,
		gateway.NewTraceServer(forwarder, gateway.WithRateLimiter(limiter)))

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalf("no se pudo escuchar en %s: %v", listen, err)
	}
	for name, lim := range plans {
		if lim.Unlimited() {
			log.Printf("plan %s: sin límite", name)
		} else {
			log.Printf("plan %s: %.0f spans/s (burst %.0f)", name, lim.SpansPerSecond, lim.Burst)
		}
	}
	log.Printf("Ingestion Gateway escuchando en %s -> downstream %s", listen, downstream)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("servidor detenido: %v", err)
	}
}
