# Compliance Engine (Python) — E5-T01 / E5-T02

Motor de mapeo regulatorio: traduce las **trazas** de los agentes en **evidencia
de cumplimiento** frente a marcos regulatorios. Incluye el mapeo del **EU AI Act**
(el gancho de producto de AgentLens).

## Modelo de mapeo (E5-T01)

- `Requirement`: un punto de checklist (id, framework, artículo, título, severidad).
- `MappedRequirement`: un requisito + una *comprobación* (función pura sobre los
  hechos derivados de las trazas).
- `Mapping`: conjunto **versionado** de requisitos de un framework (un cambio
  normativo = versión nueva). Estructura editable (datos + funciones).
- `Facts`: hechos derivados de los spans (registro, timestamps, modelo, tokens,
  entradas/salidas, herramientas, PII redactada, supervisión humana, etc.).
- `evaluate(spans, mapping) -> Report`: estado por requisito
  (`covered`/`partial`/`missing`), evidencias y **cobertura** agregada.

## Mapeo EU AI Act (E5-T02)

`frameworks/eu_ai_act.py` define un **checklist de 40 puntos** (versión
`2024-1689`) sobre:

- **Art. 12** — conservación de registros / logs automáticos
- **Art. 14** — supervisión humana
- **Art. 50** — transparencia
- **Art. 86** — derecho a explicación de decisiones
- Transversal — minimización de PII y trazabilidad

Cada punto se marca cubierto si las trazas aportan la evidencia correspondiente.

## Mapeo GDPR (E5-T03)

`frameworks/gdpr.py` mapea principios y obligaciones del RGPD verificables desde
las trazas: base legal y finalidad (Art. 5), minimización y limitación del plazo,
registro de actividades (Art. 30), derecho de acceso (Art. 15), supresión
(Art. 17) y detección de incidencias (Art. 33).

## Uso

```python
from agentlens_compliance import evaluate
from agentlens_compliance.frameworks import eu_ai_act

report = evaluate(spans, eu_ai_act.mapping())
print(report.coverage(), report.missing)        # p.ej. 0.85, 6
for r in report.gaps():
    print(r.requirement.id, r.requirement.title)
```

CLI (informe JSON desde spans):

```bash
cat spans.json | python -m agentlens_compliance
```

## Tests

```bash
pip install -e ".[dev]" && pytest -q
```

Verifican los 40 puntos, la cobertura total con trazas ricas y la detección de
gaps con trazas mínimas.

## Informe PDF firmado (E5-T05)

`generate_signed_report(report, "informe.pdf", private_pem)` genera un **PDF** con el
checklist y la cobertura, y un sidecar `informe.pdf.sig.json` con el manifiesto
firmado (**Ed25519**): hash SHA-256 del PDF, cobertura, **sello de tiempo** y huella
de la clave firmante (`signer_key_sha256`).

`verify_signed_report(pdf, sidecar, trusted_public_pem)` recomputa el hash y valida
la firma con la **clave pública de confianza** de la organización emisora, obtenida
por un canal independiente. La clave pública que viaja en el sidecar es solo
informativa: si se aceptara, cualquiera podría re-firmar un informe falso con su
propia clave. Falla si cambia el PDF, el manifiesto o la clave firmante.

```bash
python -m agentlens_compliance --keygen firma            # firma.key (0600) + firma.pub
cat spans.json | python -m agentlens_compliance --framework eu-ai-act \
    --pdf informe.pdf --signing-key firma.key            # o AGENTLENS_SIGNING_KEY_FILE
python -m agentlens_compliance --verify informe.pdf --pubkey firma.pub   # exit 0/1
```

Limitaciones: el sello de tiempo lo declara el firmante (no es un TSA RFC 3161) y
la custodia/rotación de la clave privada corresponde a E0-T09 (gestión de secretos).

Requiere el extra `pdf` (`pip install -e ".[pdf]"`: reportlab + cryptography). El
resto del paquete funciona sin él.

## Siguiente

- **E5-T04**: mapeo ISO/IEC 42001 y NIST AI RMF.
- **E5-T07**: gestión de evidencias (traza ↔ requisito ↔ informe).
