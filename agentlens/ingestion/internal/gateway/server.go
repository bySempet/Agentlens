// Package gateway implementa el servicio OTLP de trazas del Ingestion Gateway
// (E2-T05). La autenticación la resuelve el interceptor de auth; aquí se sella
// el tenant en el Resource de cada span (para que un cliente no pueda hacerse
// pasar por otro tenant) y se reenvía al downstream.
package gateway

import (
	"context"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/forward"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// TenantResourceAttr es el atributo de Resource donde el gateway sella el tenant
// autoritativo resuelto desde la API key. Coincide con el que emite el SDK.
const TenantResourceAttr = "agentlens.tenant.id"

// TraceServer implementa coltracepb.TraceServiceServer.
type TraceServer struct {
	coltracepb.UnimplementedTraceServiceServer
	forwarder forward.Forwarder
}

// NewTraceServer crea el servicio con el forwarder dado.
func NewTraceServer(forwarder forward.Forwarder) *TraceServer {
	return &TraceServer{forwarder: forwarder}
}

// Export recibe las trazas, sella el tenant autoritativo en cada ResourceSpans y
// las reenvía. Confía en que el interceptor de auth ya validó la API key y dejó
// el tenant en el contexto.
func (s *TraceServer) Export(
	ctx context.Context,
	req *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	if tenantID, ok := auth.TenantFromContext(ctx); ok {
		stampTenant(req, tenantID)
	}
	if err := s.forwarder.Forward(ctx, req); err != nil {
		return nil, err
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
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
