"""Externalización de payloads.

Patrón de producción recomendado por OpenTelemetry (2026): los prompts y
outputs grandes no viajan inline en el span. Se almacenan fuera (cifrados) y el
span guarda solo una referencia. Esto resuelve a la vez coste de
almacenamiento, privacidad y políticas de retención diferenciadas.

En el MVP el almacén es local (para desarrollo). En producción se sustituye por
un ``S3PayloadStore`` sin tocar el resto del SDK gracias a la interfaz común.
"""
from __future__ import annotations

import hashlib
import os
import uuid
from abc import ABC, abstractmethod
from typing import Tuple


class PayloadStore(ABC):
    """Interfaz de almacén de payloads externalizados."""

    @abstractmethod
    def put(self, payload: str) -> str:
        """Almacena el payload y devuelve una URI de referencia."""


class NoopPayloadStore(PayloadStore):
    """Descarta el payload (modo 'none': solo metadatos)."""

    def put(self, payload: str) -> str:  # noqa: D401
        return "agentlens://payload/discarded"


class LocalFilePayloadStore(PayloadStore):
    """Almacén local en disco. Solo para desarrollo/tests."""

    def __init__(self, directory: str = "/tmp/agentlens-payloads"):
        self.directory = directory
        os.makedirs(self.directory, exist_ok=True)

    def put(self, payload: str) -> str:
        pid = uuid.uuid4().hex
        path = os.path.join(self.directory, f"{pid}.txt")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(payload)
        return f"agentlens://payload/{pid}"


def externalize_attributes(
    attributes: dict,
    store: PayloadStore,
    content_attrs: Tuple[str, ...],
    threshold_bytes: int,
) -> dict:
    """Devuelve un nuevo dict con los payloads grandes externalizados.

    Para cada atributo de contenido que supere el umbral, sustituye su valor por
    una URI de referencia y marca el span con ``agentlens.payload.externalized``.
    """
    out = dict(attributes)
    externalized = []
    for key in content_attrs:
        value = out.get(key)
        if isinstance(value, str) and len(value.encode("utf-8")) > threshold_bytes:
            ref = store.put(value)
            digest = hashlib.sha256(value.encode("utf-8")).hexdigest()
            out[key] = ref
            out[f"agentlens.payload.{key}.sha256"] = digest
            externalized.append(key)
    if externalized:
        out["agentlens.payload.externalized"] = ",".join(externalized)
    return out


def discard_content(attributes: dict, content_attrs: Tuple[str, ...]) -> dict:
    """Modo 'none': elimina por completo los atributos de contenido."""
    return {k: v for k, v in attributes.items() if k not in content_attrs}
