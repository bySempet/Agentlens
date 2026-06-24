package gateway

import (
	"context"
	"net"
	"testing"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// captureForwarder guarda la última petición reenviada (doble de test).
type captureForwarder struct {
	last *coltracepb.ExportTraceServiceRequest
	n    int
}

func (c *captureForwarder) Forward(_ context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	c.last = req
	c.n++
	return nil
}
func (c *captureForwarder) Close() error { return nil }

// startServer arranca un gateway gRPC real sobre bufconn con la interceptación
// de auth y el forwarder dado. Devuelve un cliente y una función de cierre.
func startServer(t *testing.T, fwd *captureForwarder) (coltracepb.TraceServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	store := auth.NewStaticKeyStore(map[string]string{"key-acme": "acme"})
	srv := grpc.NewServer(grpc.UnaryInterceptor(auth.UnaryInterceptor(store)))
	coltracepb.RegisterTraceServiceServer(srv, NewTraceServer(fwd))
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	return coltracepb.NewTraceServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
	}
}

// sampleReq construye una petición con un span y, opcionalmente, un tenant que
// el cliente intenta declarar (para verificar el anti-spoofing).
func sampleReq(clientTenant string) *coltracepb.ExportTraceServiceRequest {
	res := &resourcepb.Resource{}
	if clientTenant != "" {
		res.Attributes = []*commonpb.KeyValue{{
			Key:   TenantResourceAttr,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: clientTenant}},
		}}
	}
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: res,
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{Name: "invoke_agent support-bot"}},
			}},
		}},
	}
}

func exportWithKey(client coltracepb.TraceServiceClient, key string, req *coltracepb.ExportTraceServiceRequest) error {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{auth.MetadataKey: key}))
	_, err := client.Export(ctx, req)
	return err
}

func tenantAttr(req *coltracepb.ExportTraceServiceRequest) string {
	for _, kv := range req.GetResourceSpans()[0].GetResource().GetAttributes() {
		if kv.GetKey() == TenantResourceAttr {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func TestExport_ValidKeyIsRoutedAndTenantStamped(t *testing.T) {
	fwd := &captureForwarder{}
	client, stop := startServer(t, fwd)
	defer stop()

	if err := exportWithKey(client, "key-acme", sampleReq("")); err != nil {
		t.Fatalf("clave válida rechazada: %v", err)
	}
	if fwd.n != 1 {
		t.Fatalf("se esperaba 1 reenvío, hubo %d", fwd.n)
	}
	if got := tenantAttr(fwd.last); got != "acme" {
		t.Fatalf("tenant sellado esperado acme, obtenido %q", got)
	}
}

func TestExport_InvalidKeyIsRejectedAndNotRouted(t *testing.T) {
	fwd := &captureForwarder{}
	client, stop := startServer(t, fwd)
	defer stop()

	err := exportWithKey(client, "key-falsa", sampleReq(""))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperado Unauthenticated, obtenido %v", err)
	}
	if fwd.n != 0 {
		t.Fatalf("una clave inválida no debe reenviarse; reenvíos=%d", fwd.n)
	}
}

func TestExport_ClientTenantCannotBeSpoofed(t *testing.T) {
	fwd := &captureForwarder{}
	client, stop := startServer(t, fwd)
	defer stop()

	// El cliente (autenticado como acme) intenta declararse como otro tenant.
	if err := exportWithKey(client, "key-acme", sampleReq("globex")); err != nil {
		t.Fatalf("clave válida rechazada: %v", err)
	}
	if got := tenantAttr(fwd.last); got != "acme" {
		t.Fatalf("el gateway debe sobreescribir el tenant a acme, obtenido %q", got)
	}
}
