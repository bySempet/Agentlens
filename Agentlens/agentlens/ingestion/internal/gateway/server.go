// Package gateway implementa el servicio OTLP de trazas del Ingestion Gateway
// (E2-T05, E2-T06). La autenticación la resuelve el interceptor de auth; aquí
// se aplica el límite de ingesta del plan del tenant, se sella el tenant en el
// Resource de cada span (para que un cliente no pueda hacerse pasar por otro
// tenant) y se reenvía al downstream.
package gateway

import (
	"context"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/forward"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TenantResourceAttr es el atributo de Resource donde el gateway sella el tenant
// autoritativo resuelto desde la API key. Coincide con el que emite el SDK.
const TenantResourceAttr = "agentlens.tenant.id"

// RateLimiter decide si un tenant puede ingerir `spans` spans ahora, según su
// plan (E2-T06). La implementa ratelimit.TenantLimiter; la interfaz permite un
// limitador distribuido (Redis, E2-T12) sin tocar el servidor.
type RateLimiter interface {
	Allow(tenantID, plan string, spans int) bool
}

// TraceServer implementa coltracepb.TraceServiceServer.
type TraceServer struct {
	coltracepb.UnimplementedTraceServiceServer
	forwarder forward.Forwarder
	limiter   RateLimiter // nil = sin límite de ingesta
}

// Option configura el TraceServer.
type Option func(*TraceServer)

// WithRateLimiter activa el límite de ingesta por plan (E2-T06).
func WithRateLimiter(l RateLimiter) Option {
	return func(s *TraceServer) { s.limiter = l }
}

// NewTraceServer crea el servicio con el forwarder dado.
func NewTraceServer(forwarder forward.Forwarder, opts ...Option) *TraceServer {
	s := &TraceServer{forwarder: forwarder}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Export recibe las trazas, aplica el límite de ingesta del plan, sella el
// tenant autoritativo en cada ResourceSpans y las reenvía. Confía en que el
// interceptor de auth ya validó la API key y dejó el tenant en el contexto.
func (s *TraceServer) Export(
	ctx context.Context,
	req *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	if tenant, ok := auth.TenantInfoFromContext(ctx); ok {
		if s.limiter != nil && !s.limiter.Allow(tenant.ID, tenant.Plan, countSpans(req)) {
			// ResourceExhausted: los exportadores OTLP lo tratan como
			// reintentable con backoff, no como error fatal.
			return nil, status.Errorf(codes.ResourceExhausted,
				"límite de ingesta del plan %q excedido para el tenant %q", tenant.Plan, tenant.ID)
		}
		stampTenant(req, tenant.ID)
	}
	if err := s.forwarder.Forward(ctx, req); err != nil {
		return nil, err
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

// countSpans devuelve el número total de spans de la petición (el coste que se
// cobra al bucket del tenant).
func countSpans(req *coltracepb.ExportTraceServiceRequest) int {
	n := 0
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			n += len(ss.GetSpans())
		}
	}
	return n
}

// stampTenant fuerza el atributo de tenant en el Resource de cada conjunto de
// spans con el valor autoritativo, sobreescribiendo cualquier valor que el
// cliente hubiera puesto (anti-spoofing multi-tenant).
func stampTenant(req *coltracepb.ExportTraceServiceRequest, tenantID string) {
	for _, rs := range req.GetResourceSpans() {
		if rs.Resource == nil {
			rs.Resource = &resourcepb.Resource{}
		}
		setStringAttr(&rs.Resource.Attributes, TenantResourceAttr, tenantID)
	}
}

// setStringAttr inserta o reemplaza un atributo string en la lista de KeyValue.
func setStringAttr(attrs *[]*commonpb.KeyValue, key, value string) {
	kv := &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	}
	for i, existing := range *attrs {
		if existing.GetKey() == key {
			(*attrs)[i] = kv
			return
		}
	}
	*attrs = append(*attrs, kv)
}
