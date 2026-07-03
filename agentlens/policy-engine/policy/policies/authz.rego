# Política de autorización base de AgentLens (E4-T01).
#
# Decide si una acción de un agente se permite. Acumula motivos de denegación en
# `deny`; `allow` es verdadero solo si no hay denegaciones. Los umbrales son
# data-driven (data.config.*), así que el control-plane puede ajustarlos sin
# recompilar la política.
package agentlens.authz

import rego.v1

default allow := false

# Herramientas peligrosas explícitamente bloqueadas.
deny contains msg if {
	some tool in data.config.blocked_tools
	input.tool == tool
	msg := sprintf("herramienta bloqueada: %v", [input.tool])
}

# Minimización de datos: PII no puede salir a un destino externo.
deny contains msg if {
	input.pii_detected == true
	input.destination == "external"
	msg := "salida con PII hacia destino externo bloqueada"
}

# Límite de tokens de salida por acción (control de coste/abuso).
deny contains msg if {
	limit := data.config.limits.max_output_tokens
	input.output_tokens > limit
	msg := sprintf("supera el límite de tokens de salida (%v > %v)", [input.output_tokens, limit])
}

allow if count(deny) == 0
