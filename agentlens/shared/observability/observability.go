// Package observability arranca un TracerProvider OTLP para que los servicios de
// AgentLens se observen a sí mismos con OpenTelemetry (E0-T08, dogfooding).
//
// Es no-op si AGENTLENS_OTLP_ENDPOINT no está configurado, así no rompe la
// ejecución local sin Collector.
package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown vacía y cierra el provider. Siempre es seguro llamarla.
type Shutdown func(context.Context) error

// Init configura el TracerProvider global del servicio. Devuelve una función de
// cierre. Si no hay endpoint OTLP, instala un no-op.
func Init(ctx context.Context, serviceName string) (Shutdown, error) {
	endpoint := os.Getenv("AGENTLENS_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		attribute.String("agentlens.component", "platform"),
	))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
