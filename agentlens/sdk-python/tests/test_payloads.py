from agentlens.payloads import (
    LocalFilePayloadStore,
    discard_content,
    externalize_attributes,
)


def test_externalizes_large_payload(tmp_path):
    store = LocalFilePayloadStore(directory=str(tmp_path))
    big = "x" * 5000
    attrs = {"gen_ai.output.messages": big, "gen_ai.request.model": "gpt-4o"}
    out = externalize_attributes(
        attrs, store, ("gen_ai.output.messages",), threshold_bytes=4096
    )
    assert out["gen_ai.output.messages"].startswith("agentlens://payload/")
    assert "agentlens.payload.externalized" in out
    assert "gen_ai.output.messages" in out["agentlens.payload.externalized"]
    assert out["gen_ai.request.model"] == "gpt-4o"  # los metadatos se quedan


def test_small_payload_stays_inline(tmp_path):
    store = LocalFilePayloadStore(directory=str(tmp_path))
    attrs = {"gen_ai.output.messages": "hola"}
    out = externalize_attributes(
        attrs, store, ("gen_ai.output.messages",), threshold_bytes=4096
    )
    assert out["gen_ai.output.messages"] == "hola"
    assert "agentlens.payload.externalized" not in out


def test_discard_content_mode():
    attrs = {"gen_ai.output.messages": "secreto", "gen_ai.request.model": "gpt-4o"}
    out = discard_content(attrs, ("gen_ai.output.messages",))
    assert "gen_ai.output.messages" not in out
    assert out["gen_ai.request.model"] == "gpt-4o"
