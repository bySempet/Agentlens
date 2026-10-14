"""Firma y verificación de informes de compliance (E5-T05).

Firma un *manifiesto* (framework, versión, cobertura, sello de tiempo y hash
SHA-256 del PDF) con Ed25519. La verificación recomputa el hash del PDF y
comprueba la firma contra una **clave pública de confianza** que aporta quien
verifica: si el PDF o el manifiesto cambian, o si lo firmó otra clave, falla.
"""
from __future__ import annotations

import hashlib
import json
from typing import Any, Dict, Tuple

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey,
    Ed25519PublicKey,
)


def generate_keypair() -> Tuple[bytes, bytes]:
    """Genera un par de claves Ed25519 (private_pem, public_pem)."""
    priv = Ed25519PrivateKey.generate()
    priv_pem = priv.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    pub_pem = priv.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )
    return priv_pem, pub_pem


def public_key_fingerprint(public_pem: bytes) -> str:
    """Huella SHA-256 de la clave pública Ed25519 (32 bytes crudos), en hex."""
    pub = serialization.load_pem_public_key(public_pem)
    if not isinstance(pub, Ed25519PublicKey):
        raise TypeError("se esperaba una clave pública Ed25519")
    raw = pub.public_bytes(
        encoding=serialization.Encoding.Raw, format=serialization.PublicFormat.Raw
    )
    return hashlib.sha256(raw).hexdigest()


def public_pem_from_private(private_pem: bytes) -> bytes:
    priv = serialization.load_pem_private_key(private_pem, password=None)
    if not isinstance(priv, Ed25519PrivateKey):
        raise TypeError("se esperaba una clave privada Ed25519")
    return priv.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )


def sha256_file(path: str) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def _canonical(manifest: Dict[str, Any]) -> bytes:
    """Serialización canónica y estable del manifiesto para firmar/verificar."""
    return json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode("utf-8")


def sign_manifest(manifest: Dict[str, Any], private_pem: bytes) -> str:
    priv = serialization.load_pem_private_key(private_pem, password=None)
    if not isinstance(priv, Ed25519PrivateKey):
        raise TypeError("se esperaba una clave privada Ed25519")
    return priv.sign(_canonical(manifest)).hex()


def verify_manifest(manifest: Dict[str, Any], signature_hex: str, public_pem: bytes) -> bool:
    pub = serialization.load_pem_public_key(public_pem)
    if not isinstance(pub, Ed25519PublicKey):
        raise TypeError("se esperaba una clave pública Ed25519")
    try:
        pub.verify(bytes.fromhex(signature_hex), _canonical(manifest))
        return True
    except (InvalidSignature, ValueError):
        return False
