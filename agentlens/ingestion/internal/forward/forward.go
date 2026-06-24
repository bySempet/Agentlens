// Package forward enruta las trazas ya autenticadas hacia el downstream
// (el OTel Collector en el MVP). El gateway no almacena: valida, sella el tenant
// y reenvía. La interfaz Forwarder permite cambiar el destino (Collector, Kafka)
// sin tocar el servidor, y usar un doble en tests.
package forward

import (
	"context"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Forwarder reenvía una petición OTLP de trazas al siguiente salto.
type Forwarder interface {
	Forward(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error
	Close() error
}

// OTLPForwarder reenvía vía OTLP/gRPC a un Collector downstream.
type OTLPForwarder struct {
	conn   *grpc.ClientConn
	client coltracepb.TraceServiceClient
}

// NewOTLPForwarder abre una conexión gRPC al endpoint downstream. En el MVP el
// canal interno gateway->collector es de confianza (red privada), por eso usa
// credenciales inseguras; el TLS está en el borde (cliente -> gateway).
func NewOTLPForwarder(endpoint string) (*OTLPForwarder, error) {
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &OTLPForwarder{conn: conn, client: coltracepb.NewTraceServiceClient(conn)}, nil
}

// Forward implementa Forwarder.
func (f *OTLPForwarder) Forward(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	_, err := f.client.Export(ctx, req)
	return err
}

// Close cierra la conexión gRPC subyacente.
func (f *OTLPForwarder) Close() error {
	if f.conn != nil {
		return f.conn.Close()
	}
	return nil
}
