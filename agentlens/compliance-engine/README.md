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

## Siguiente

- **E5-T03**: mapeo GDPR. **E5-T05**: generador de informes PDF firmados.
  **E5-T06**: dashboard de madurez de compliance.
