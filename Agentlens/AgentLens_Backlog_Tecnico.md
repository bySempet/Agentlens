# AgentLens — Backlog Técnico de Ingeniería

**Documento de ingeniería** · Junio 2026 · Derivado del Plan Técnico de Desarrollo

---

## Leyenda

- **Estimación:** S ≤ 2 días · M = 3–5 días · L = 1–2 semanas · XL > 2 semanas
- **Prioridad:** P0 = crítico para el MVP / fase actual · P1 = siguiente · P2 = posterior
- **ID:** `Ex-Tyy` (épica x, tarea yy). Las dependencias referencian otros IDs.

Las épicas E0–E3 forman el **MVP (v0.1, M1–M3)**. El desarrollo real arranca por E1 (SDK), que es el camino crítico y el gancho open-source.

---

## E0 — Fundamentos de ingeniería e infraestructura

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E0-T01 | Crear monorepo con estructura de paquetes (sdk-python, sdk-node, collector, ingestion, frontend, deploy, policies) | `git clone` + build de cada paquete vacío pasa en CI | — | S | P0 |
| E0-T02 | Pipeline CI con GitHub Actions (lint, test, build por paquete) | PR ejecuta lint+test+build de los paquetes afectados | E0-T01 | M | P0 |
| E0-T03 | Gate de latencia del SDK en CI (falla si overhead > 5 ms) | Test de benchmark integrado que rompe el build | E1-T03 | M | P0 |
| E0-T04 | Entorno de desarrollo local (docker-compose: Collector + ClickHouse) | `docker compose up` levanta el stack y recibe trazas | E2-T01 | M | P0 |
| E0-T05 | IaC base (Terraform) para nube AgentLens en EU (VPC, EKS, RDS, S3, MSK) | `terraform apply` provisiona el entorno de staging EU | — | L | P1 |
| E0-T06 | GitOps con ArgoCD + Helm charts base de cada servicio | Cambio en `main` despliega a staging automáticamente | E0-T05 | L | P1 |
| E0-T07 | Despliegues blue/green con rollback automático | Rollout sin downtime verificado en staging | E0-T06 | M | P1 |
| E0-T08 | Observabilidad interna (la plataforma se monitoriza a sí misma con OTel) | Métricas y trazas de los servicios AgentLens visibles | E0-T05 | M | P1 |
| E0-T09 | Gestión de secretos (Vault / AWS Secrets Manager) | Ningún secreto en repo; rotación documentada | E0-T05 | S | P1 |

## E1 — Core Tracing SDK (Python + TypeScript) · v0.1

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E1-T01 | Esqueleto del paquete `agentlens` (Python) con `instrument()` de 3 líneas | `pip install` + `instrument()` configura un TracerProvider | E0-T01 | S | P0 |
| E1-T02 | Export OTLP/gRPC asíncrono con BatchSpanProcessor (flush por batch) | Spans llegan a un Collector sin bloquear el hilo del agente | E1-T01 | S | P0 |
| E1-T03 | Buffer en memoria + flush cada 100 ms; cero espera de confirmación | Benchmark: overhead < 5 ms por span en el hot path | E1-T02 | M | P0 |
| E1-T04 | Redacción de PII en cliente (email, IBAN, tarjeta, teléfono; extensible) | Atributos sensibles mascarados antes de exportar; unit tests | E1-T01 | M | P0 |
| E1-T05 | Modo de externalización de payloads (`reference`): payloads grandes fuera del span | Spans grandes llevan `app.payload.ref` en vez del contenido inline | E1-T04 | M | P0 |
| E1-T06 | Enriquecimiento de spans (tenant_id, agent_id, service.name) vía Resource | Todos los spans llevan los atributos de tenant/agente | E1-T01 | S | P0 |
| E1-T07 | Auto-instrumentación OpenAI (Python) | Llamadas a OpenAI generan client spans gen_ai.* sin código extra | E1-T01 | M | P0 |
| E1-T08 | Auto-instrumentación LangChain / LangGraph | Cadenas LangChain generan spans de agente y herramienta | E1-T07 | M | P0 |
| E1-T09 | Capa adaptadora de convenciones (aísla esquema interno del OTel GenAI *Development*) | Cambio de versión de semconv no rompe el contrato del SDK | E1-T07 | L | P0 |
| E1-T10 | Auto-instrumentación CrewAI, AutoGen, Pydantic AI, Bedrock Agents | Cada framework P0 emite spans correctos; matriz de tests | E1-T08, E1-T09 | L | P1 |
| E1-T11 | SDK TypeScript `@agentlens/node` (paridad funcional con Python) | `npm i` + setup de 3 líneas; spans desde agente Node | E1-T09 | L | P1 |
| E1-T12 | Trazado MCP (gen_ai.tool.name, mcp.server.name, propagación de contexto) | Tool calls MCP aparecen como spans correlacionados | E1-T11 | M | P1 |
| E1-T13 | Documentación + README open-source + tutorial de integración | Integración en 5 pasos reproducible por un externo | E1-T08 | S | P0 |
| E1-T14 | Publicación open-source (GitHub público + PyPI + npm) | Paquete instalable públicamente; primer release etiquetado | E1-T13 | S | P0 |

## E2 — Pipeline de ingesta, procesamiento y almacenamiento · v0.1

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E2-T01 | OTel Collector con receptor OTLP + config base | Collector recibe OTLP del SDK y exporta a backend de debug | E0-T01 | S | P0 |
| E2-T02 | Procesador custom de redacción en Collector (Presidio) | Segunda barrera de PII verificada con datos de prueba | E2-T01 | M | P0 |
| E2-T03 | Procesador de externalización de payloads (push a S3, deja referencia) | Payloads grandes en S3 cifrado; span queda con ref | E2-T01 | M | P1 |
| E2-T04 | Tail sampling (100% errores/lentos, muestreo del resto) | Reglas de sampling configurables; verificadas | E2-T01 | M | P1 |
| E2-T05 | Ingestion Gateway (Go): OTLP/gRPC + TLS + auth por API key | Rechaza claves inválidas; acepta y rutea las válidas | E0-T01 | M | P0 |
| E2-T06 | Resolución de tenant + rate limiting por plan | Límites por tier aplicados; tests de carga | E2-T05 | M | P0 |
| E2-T07 | Esquema ClickHouse para spans/trazas + ingestión | Trazas consultables con queries sub-segundo | E2-T01 | M | P0 |
| E2-T08 | Esquema PostgreSQL (agentes, políticas, orgs, usuarios) + migraciones | Migraciones versionadas; modelo ACID | E0-T01 | M | P0 |
| E2-T09 | Kafka (MSK/Confluent) entre gateway y procesamiento | Eventos durables con replay; verificado | E2-T05 | M | P1 |
| E2-T10 | Stream processor (Go): enriquecimiento + disparo de evaluación de políticas | Eventos enriquecidos en tiempo real | E2-T09 | L | P1 |
| E2-T11 | Archivo frío S3 + Parquet + Iceberg (retención hasta 7 años) | Query SQL sobre archivo histórico vía Trino/Athena | E2-T03 | L | P2 |
| E2-T12 | Redis (Valkey) para sesiones, rate limit y cache de decisiones | Cache funcional con TTL; tests | E2-T06 | S | P1 |
| E2-T13 | Métricas time-series (VictoriaMetrics): latencia, coste tokens, error rate | Dashboards de métricas por agente | E2-T07 | M | P1 |

## E3 — Dashboard de trazas + API hot path · v0.1

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E3-T01 | API hot path (Go) de lectura de trazas sobre ClickHouse | Endpoints de listado/detalle de trazas con paginación | E2-T07 | M | P0 |
| E3-T02 | API REST/GraphQL para dashboard e integraciones | Esquema documentado; auth por tenant | E3-T01 | M | P0 |
| E3-T03 | Dashboard Next.js: lista y detalle de trazas (timeline de spans) | Usuario ve la traza paso a paso de un agente | E3-T02 | L | P0 |
| E3-T04 | Vista de conversación GenAI (prompts/outputs/tool calls legibles) | Render chat-style de los atributos gen_ai.* | E3-T03 | M | P0 |
| E3-T05 | Live feed por WebSocket de actividad de agentes | Eventos nuevos aparecen en tiempo real | E3-T01 | M | P1 |
| E3-T06 | Dashboard de coste por agente/tarea/modelo | Coste de tokens agregado y filtrable | E2-T13 | M | P1 |
| E3-T07 | Inventario y registro de agentes (CRUD) | Alta/baja de agentes; genera agent_id | E2-T08 | M | P0 |

## E4 — Policy Engine y enforcement en tiempo real · v0.2

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E4-T01 | Servicio OPA + librería de políticas Rego base | Evaluación de política de prueba < 5 ms | E2-T08 | M | P0 |
| E4-T02 | Compilación de políticas a bundle WASM | Bundle WASM generado desde Rego en CI | E4-T01 | M | P0 |
| E4-T03 | Enforcement síncrono en el borde (SDK/gateway evalúa WASM local) | Acción bloqueada localmente en sub-ms sin llamar al backend | E4-T02, E1-T03 | L | P0 |
| E4-T04 | Sincronización de políticas control-plane → borde | Cambio de política se propaga a los SDKs/gateways | E4-T03 | M | P0 |
| E4-T05 | Escalado automático a supervisión humana por umbral | Acción sobre umbral abre intervención humana | E4-T03 | M | P1 |
| E4-T06 | UI de gestión de políticas | Cliente crea/edita políticas sin tocar código | E4-T01, E3-T03 | M | P1 |
| E4-T07 | Integración de alertas Slack / Microsoft Teams | Violación de política dispara alerta | E4-T05 | S | P1 |
| E4-T08 | Detección de anomalías en stream (base estadística) | Comportamiento anómalo marcado en tiempo real | E2-T10 | L | P1 |

## E5 — Compliance Reporter · v0.3

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E5-T01 | Modelo de mapeo regulatorio (traza → requisito) versionado | Estructura de mapeo editable y versionada | E2-T08 | M | P0 |
| E5-T02 | Mapeo EU AI Act (Art. 12 logs, Art. 14 supervisión, Art. 50, Art. 86) | Checklist de 40 puntos cubierto por trazas reales | E5-T01 | L | P0 |
| E5-T03 | Mapeo GDPR (base legal, minimización, logs de acceso) | Evidencias GDPR generadas desde trazas | E5-T01 | M | P0 |
| E5-T04 | Mapeo ISO/IEC 42001 y NIST AI RMF | Informe de conformidad por framework | E5-T01 | M | P1 |
| E5-T05 | Generador de informes PDF/Word firmados con sello de tiempo | Informe descargable, firmado y verificable | E5-T02 | M | P0 |
| E5-T06 | Dashboard de madurez de compliance | Puntuación de madurez por framework | E5-T02, E3-T03 | M | P1 |
| E5-T07 | Gestión de evidencias (vinculación traza ↔ requisito ↔ informe) | Trazabilidad completa de cada evidencia | E5-T05 | M | P1 |

## E6 — AI Forensics · v0.4

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E6-T01 | Reconstrucción de ejecución desde replay de Kafka + ClickHouse | Ejecución reproducida paso a paso de extremo a extremo | E2-T09, E2-T07 | L | P0 |
| E6-T02 | Captura/restauración de estado de memoria y herramientas por paso | Cada paso muestra memoria, tools y datos accedidos | E6-T01 | L | P0 |
| E6-T03 | Timeline forense interactivo en el dashboard | Navegación temporal de la ejecución del agente | E6-T02, E3-T03 | M | P1 |
| E6-T04 | Export forense firmado criptográficamente (uso legal) | Paquete exportable verificable e inalterable | E6-T02 | M | P0 |
| E6-T05 | Cadena de custodia y verificación de integridad | Hash encadenado verificable de la evidencia | E6-T04 | M | P1 |

## E7 — Enterprise Memory · v0.5

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E7-T01 | Capa de contexto compartido entre agentes | Agentes leen/escriben memoria compartida | E2-T12 | L | P0 |
| E7-T02 | RBAC granular sobre la memoria | Acceso por rol; agente sin permiso no lee | E7-T01 | M | P0 |
| E7-T03 | Búsqueda semántica (pgvector / Qdrant) sobre decisiones previas | Recuperación vectorial de contexto relevante | E7-T01 | M | P1 |
| E7-T04 | Políticas de retención y expiración de memoria | Memoria caduca según política configurada | E7-T01 | S | P1 |
| E7-T05 | Auditoría de acceso a memoria (quién, qué, cuándo) | Log completo de accesos; detecta contaminación cruzada | E7-T02 | M | P0 |

## E8 — Guardian Agents · v1.0

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E8-T01 | Meta-agente que monitoriza a otros agentes | Guardian observa y reporta sobre agentes objetivo | E4-T08 | L | P0 |
| E8-T02 | Detección de comportamiento anómalo con ML | Modelo detecta desviaciones sobre baseline | E8-T01, E2-T13 | XL | P0 |
| E8-T03 | Intervención automática configurable | Guardian bloquea/escala según configuración | E8-T02, E4-T03 | M | P1 |
| E8-T04 | Predicción de riesgo previa al incidente | Alerta antes de que ocurra el fallo | E8-T02 | XL | P2 |

## E9 — Seguridad, multi-tenancy y certificaciones (transversal)

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E9-T01 | Aislamiento multi-tenant (namespace K8s + schema/db en ClickHouse y Postgres) | Datos de dos tenants jamás se mezclan; tests | E2-T07, E2-T08 | L | P0 |
| E9-T02 | Auth empresarial: Auth0/Clerk + SAML/OIDC + MFA | SSO funcional desde plan Growth | E2-T08 | M | P0 |
| E9-T03 | SCIM provisioning (Enterprise) | Alta/baja de usuarios automatizada | E9-T02 | M | P1 |
| E9-T04 | Cifrado en reposo y en tránsito de extremo a extremo | Todo el dato cifrado; verificado | E0-T05 | M | P0 |
| E9-T05 | Pen testing trimestral + remediación | Primer pen test cerrado sin críticos abiertos | E9-T01 | M | P0 |
| E9-T06 | Certificación SOC 2 Type II | Auditoría superada | E9-T05 | XL | P0 |
| E9-T07 | Certificación ISO 27001 | Certificado obtenido | E9-T06 | XL | P1 |
| E9-T08 | Certificación ISO/IEC 42001 (diferenciador EU AI Act) | Certificado obtenido | E9-T07 | XL | P1 |
| E9-T09 | Residencia de datos EU por defecto (eu-west-1 / eu-central-1) | Ningún dato de cliente UE sale de la UE | E0-T05 | M | P0 |

## E10 — Free tier, onboarding y PLG

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E10-T01 | Registro self-serve (< 2 min) | Alta de cuenta y tenant sin intervención manual | E9-T02 | M | P0 |
| E10-T02 | Free tier con límites (50K acciones/mes, 1 agente, 7 días retención) | Límites aplicados y visibles | E2-T06, E10-T01 | M | P0 |
| E10-T03 | Onboarding guiado (tiempo hasta primer trace < 10 min) | Métrica TTV medida en el funnel | E1-T13, E3-T03 | M | P0 |
| E10-T04 | Facturación y planes (Stripe) + upgrade Free→Paid | Cobro recurrente y upgrade self-serve | E10-T01 | L | P1 |
| E10-T05 | Landing page con EU AI Act como gancho + SEO | Página publicada y medible | — | M | P0 |

## E11 — Topologías de despliegue (SaaS / VPC / on-prem)

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E11-T01 | Despliegue SaaS multi-tenant completo (staging→prod) | Plataforma operativa en nube AgentLens EU | E0-T06, E9-T01 | L | P0 |
| E11-T02 | Modo híbrido/VPC: Collector + ClickHouse en VPC del cliente | Trazas crudas no salen de la cuenta del cliente | E11-T01 | L | P1 |
| E11-T03 | Modo on-prem/air-gapped: stack completo vía Helm sin egress | Instalación reproducible sin conexión externa | E11-T02 | XL | P2 |
| E11-T04 | Bundles firmados de políticas y mapeos regulatorios para air-gapped | Cliente importa actualizaciones offline verificables | E11-T03, E5-T01 | M | P2 |

## E12 — Integraciones del ecosistema (P1–P2)

| ID | Tarea | Criterio de aceptación | Dep. | Est. | Pri. |
| --- | --- | --- | --- | --- | --- |
| E12-T01 | Jira / Linear: apertura automática de tickets en incidentes | Incidente crea ticket con contexto | E4-T07 | S | P1 |
| E12-T02 | PagerDuty / OpsGenie: alertas críticas de compliance | Alerta crítica notifica on-call | E4-T07 | S | P1 |
| E12-T03 | Splunk / Elastic: exportación de logs a SIEM del cliente | Logs exportables al SIEM existente | E2-T10 | M | P2 |
| E12-T04 | Grafana: dashboards custom sobre datos de AgentLens | Datasource funcional | E2-T13 | S | P2 |
| E12-T05 | ServiceNow: integración con GRC existente (Enterprise) | Sincronización con GRC del cliente | E5-T07 | M | P2 |

---

## Resumen de ruta crítica del MVP

```
E0-T01 → E1-T01 → E1-T02 → E1-T03 ─┐
                E1-T04 → E1-T05 ───┤
                E1-T07 → E1-T08 ───┼→ E1-T13 → E1-T14 (SDK público)
E2-T01 → E2-T07 ──────────────────┤
E2-T05 → E2-T06 ──────────────────┤
                E3-T01 → E3-T02 → E3-T03 → E3-T04 (dashboard de trazas)
E10-T01 → E10-T02 → E10-T03 (free tier + onboarding)
```

El desarrollo arranca por la rama del SDK (E1), porque es el camino crítico, el gancho open-source y la pieza que más riesgo técnico concentra (latencia + adaptador de convenciones).
