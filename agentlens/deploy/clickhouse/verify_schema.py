"""Verificación del esquema ClickHouse de AgentLens (E2-T07) con chDB.

Ejecuta `init.sql` contra una instancia ClickHouse embebida (chDB), inserta spans
de muestra usando EXACTAMENTE la lista de columnas que escribe el exporter OTel
(para probar compatibilidad), y corre las consultas del dashboard comprobando
resultados, aislamiento por tenant y latencia.

Uso:
    pip install chdb
    python deploy/clickhouse/verify_schema.py
"""
import json
import os
import time

from chdb import session as chs

INIT_SQL = os.path.join(os.path.dirname(__file__), "init.sql")


def run_statements(sess, sql):
    # Quita comentarios de línea (-- ...) antes de partir por ';', porque algunos
    # comentarios contienen ';'. clickhouse-client lo hace nativamente.
    no_comments = "\n".join(line.split("--", 1)[0] for line in sql.splitlines())
    for stmt in [s.strip() for s in no_comments.split(";")]:
        if stmt:
            sess.query(stmt)


def q(sess, sql, fmt="JSONEachRow"):
    out = sess.query(sql, fmt)
    text = out.bytes().decode() if hasattr(out, "bytes") else str(out)
    return [json.loads(line) for line in text.splitlines() if line.strip()]


def main():
    sess = chs.Session()
    with open(INIT_SQL) as fh:
        run_statements(sess, fh.read())
    print("[ok] init.sql ejecutado (schema creado)")

    cols = ("Timestamp, TraceId, SpanId, ParentSpanId, TraceState, SpanName, "
            "SpanKind, ServiceName, ResourceAttributes, ScopeName, ScopeVersion, "
            "SpanAttributes, Duration, StatusCode, StatusMessage")
    rows = [
        ("'2026-06-25 10:00:00.000'", "'T1'", "'s1'", "''", "''", "'invoke_agent support-bot'",
         "'SPAN_KIND_INTERNAL'", "'support-agent'",
         "{'agentlens.tenant.id':'acme','agentlens.agent.id':'support-bot'}",
         "'agentlens'", "'0.1.0'", "{'gen_ai.operation.name':'invoke_agent'}",
         "5000000", "'STATUS_CODE_OK'", "''"),
        ("'2026-06-25 10:00:00.001'", "'T1'", "'s2'", "'s1'", "''", "'chat gpt-4o'",
         "'SPAN_KIND_CLIENT'", "'support-agent'",
         "{'agentlens.tenant.id':'acme','agentlens.agent.id':'support-bot'}",
         "'agentlens'", "'0.1.0'",
         "{'gen_ai.operation.name':'chat','gen_ai.request.model':'gpt-4o','gen_ai.usage.input_tokens':'120','gen_ai.usage.output_tokens':'40'}",
         "3000000", "'STATUS_CODE_OK'", "''"),
        ("'2026-06-25 10:05:00.000'", "'T2'", "'s3'", "''", "''", "'invoke_agent support-bot'",
         "'SPAN_KIND_INTERNAL'", "'support-agent'",
         "{'agentlens.tenant.id':'acme','agentlens.agent.id':'support-bot'}",
         "'agentlens'", "'0.1.0'", "{'gen_ai.operation.name':'invoke_agent'}",
         "2000000", "'STATUS_CODE_ERROR'", "'tool timeout'"),
        ("'2026-06-25 10:10:00.000'", "'T3'", "'s4'", "''", "''", "'invoke_agent biller'",
         "'SPAN_KIND_INTERNAL'", "'billing-agent'",
         "{'agentlens.tenant.id':'globex','agentlens.agent.id':'biller'}",
         "'agentlens'", "'0.1.0'", "{'gen_ai.operation.name':'invoke_agent'}",
         "1000000", "'STATUS_CODE_OK'", "''"),
    ]
    values = ", ".join("(" + ", ".join(r) + ")" for r in rows)
    sess.query(f"INSERT INTO agentlens.otel_traces ({cols}) VALUES {values}")
    print(f"[ok] insertados {len(rows)} spans con la lista de columnas del exporter")

    mat = q(sess, "SELECT TenantId, AgentId, InputTokens, OutputTokens, IsError "
                  "FROM agentlens.otel_traces WHERE TraceId='T1' AND SpanId='s2'")[0]
    assert mat["TenantId"] == "acme", mat
    assert mat["InputTokens"] == 120 and mat["OutputTokens"] == 40, mat
    assert mat["IsError"] == 0, mat
    print("[ok] columnas materializadas (tenant/tokens/IsError):", mat)

    t0 = time.perf_counter()
    traces = q(sess, """
        SELECT TraceId,
               argMinMerge(RootSpanName) AS root,
               (maxMerge(EndNs) - minMerge(StartNs)) / 1e6 AS duration_ms,
               countMerge(SpanCount) AS spans,
               sumMerge(ErrorCount) AS errors,
               sumMerge(InputTokens) + sumMerge(OutputTokens) AS tokens
        FROM agentlens.trace_summary
        WHERE TenantId = 'acme'
        GROUP BY TraceId
        ORDER BY minMerge(StartNs) DESC
    """)
    dt = (time.perf_counter() - t0) * 1000
    by_id = {t["TraceId"]: t for t in traces}
    assert set(by_id) == {"T1", "T2"}, f"aislamiento de tenant roto: {set(by_id)}"
    assert by_id["T1"]["spans"] == 2 and by_id["T1"]["tokens"] == 160, by_id["T1"]
    assert by_id["T1"]["root"] == "invoke_agent support-bot", by_id["T1"]
    assert abs(by_id["T1"]["duration_ms"] - 5.0) < 0.001, by_id["T1"]
    assert by_id["T2"]["errors"] == 1, by_id["T2"]
    print(f"[ok] lista de trazas correcta y aislada por tenant ({dt:.2f} ms)")

    detail = q(sess, """
        SELECT SpanId, ParentSpanId, SpanName, GenAIOperation, RequestModel, StatusCode
        FROM agentlens.otel_traces
        WHERE TenantId = 'acme' AND TraceId = 'T1'
        ORDER BY Timestamp
    """)
    assert [d["SpanId"] for d in detail] == ["s1", "s2"], detail
    assert detail[1]["RequestModel"] == "gpt-4o", detail[1]
    print("[ok] detalle de traza (timeline de spans)")

    cost = q(sess, """
        SELECT AgentId, sum(InputTokens) AS in_tok, sum(OutputTokens) AS out_tok
        FROM agentlens.otel_traces WHERE TenantId = 'acme' GROUP BY AgentId
    """)
    assert cost[0]["in_tok"] == 120 and cost[0]["out_tok"] == 40, cost
    print("[ok] rollup de tokens por agente:", cost)

    print("\nTODAS LAS VERIFICACIONES PASARON")


if __name__ == "__main__":
    main()
