"""Enforcement de políticas en el borde (E4-T03).

Evalúa en el propio proceso del agente el ``policy.wasm`` que compila el Policy
Engine (E4-T02, ``policy-engine/cmd/buildbundle``) usando **wasmtime**: la acción
se permite o se bloquea localmente, en sub-milisegundos y sin llamar al backend.

Implementa el lado *host* del ABI WebAssembly de OPA (v1.2+):

- importa ``env.memory`` y las funciones ``opa_abort`` / ``opa_builtin0..4``;
- carga ``data.json`` del bundle una vez (``opa_json_parse``) y fija ese punto
  del heap como base;
- por evaluación escribe el input JSON en la base del heap y llama a la
  evaluación *one-shot* ``opa_eval``, que devuelve el resultado como JSON. Volver
  a partir de la misma base en cada llamada recicla el heap sin fugas.

Los builtins que OPA no compila a WASM los aporta el host; hoy la política solo
usa ``sprintf``. Si un bundle pide un builtin no soportado, la carga falla (mejor
que permitir con una política incompleta).

La evaluación es **fail-closed**: cualquier error (trap, resultado inesperado)
produce una decisión de denegación, nunca un permiso.
"""
from __future__ import annotations

import json
import re
import tarfile
import threading
import time
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, List, Mapping, Optional

DEFAULT_ENTRYPOINT = "agentlens/authz"

# Atributos de span con el resultado del enforcement.
ATTR_POLICY_ALLOW = "agentlens.policy.allow"
ATTR_POLICY_DENIALS = "agentlens.policy.denials"
ATTR_POLICY_EVAL_US = "agentlens.policy.eval_us"

_WASM_PAGE = 65536


@dataclass(frozen=True)
class PolicyDecision:
    allow: bool
    denials: List[str] = field(default_factory=list)
    eval_us: float = 0.0  # duración de la evaluación en microsegundos


class PolicyViolation(Exception):
    """La política local ha bloqueado la acción."""

    def __init__(self, decision: PolicyDecision):
        self.decision = decision
        super().__init__("acción bloqueada por política: " + "; ".join(decision.denials))


class PolicyError(Exception):
    """El bundle o el módulo WASM no se pueden cargar."""


# --- builtins del host --------------------------------------------------------

_VERB = re.compile(r"%([%vsdq])")


def _fmt_value(v: Any) -> str:
    """Aproxima el ``%v`` de Go para valores JSON (lo que usa sprintf de OPA)."""
    if isinstance(v, bool):
        return "true" if v else "false"
    if v is None:
        return "null"
    if isinstance(v, float) and v.is_integer():
        return str(int(v))
    if isinstance(v, (dict, list)):
        return json.dumps(v, separators=(",", ":"), ensure_ascii=False)
    return str(v)


def _sprintf(fmt: str, args: list) -> str:
    it = iter(args)

    def sub(m: "re.Match[str]") -> str:
        verb = m.group(1)
        if verb == "%":
            return "%"
        try:
            v = next(it)
        except StopIteration:
            return "%!" + verb + "(MISSING)"
        if verb == "q" and isinstance(v, str):
            return json.dumps(v, ensure_ascii=False)
        return _fmt_value(v)

    return _VERB.sub(sub, fmt)


_BUILTINS: Dict[str, Callable[..., Any]] = {"sprintf": _sprintf}


# --- llamadas rápidas a exports i32 ----------------------------------------------


def _i32_caller(wt: Any, store: Any, func: Any) -> Callable[..., int]:
    """Devuelve un invocador de ``func`` (solo i32 -> i32) para el hot path.

    ``Func.__call__`` de wasmtime-py reconstruye y compara los tipos de la
    función en cada llamada (~40 µs). Las firmas del ABI de OPA son todas i32,
    así que se validan una vez aquí y la llamada va directa a
    ``wasmtime_func_call`` (unos pocos µs). Si los internos de wasmtime-py no
    están disponibles, se usa la llamada pública.
    """
    ty = func.type(store)
    i32 = wt.ValType.i32()
    n_params, n_results = len(ty.params), len(ty.results)
    if any(t != i32 for t in ty.params) or n_results > 1 or any(t != i32 for t in ty.results):
        return lambda *args: func(store, *args)
    try:
        from ctypes import byref

        from wasmtime import _ffi as ffi
        from wasmtime._func import enter_wasm
        kind = ffi.WASMTIME_I32.value
        raw_func = func._func
        call = ffi.wasmtime_func_call
        params_t = ffi.wasmtime_val_t * n_params
        results_t = ffi.wasmtime_val_t * max(n_results, 1)
    except (ImportError, AttributeError):  # pragma: no cover - otra versión de wasmtime
        return lambda *args: func(store, *args)

    def fast(*args: int) -> int:
        params = params_t()
        for i, a in enumerate(args):
            params[i].kind = kind
            params[i].of.i32 = a
        results = results_t()
        with enter_wasm(store) as trap:
            err = call(store._context(), byref(raw_func), params, n_params,
                       results, n_results, trap)
            if err:
                raise wt.WasmtimeError._from_ptr(err)
        return results[0].of.i32 if n_results else 0

    return fast


# --- host del ABI de OPA --------------------------------------------------------


class WasmPolicy:
    """Política OPA compilada a WASM, evaluada localmente con wasmtime.

    Thread-safe: una instancia WASM no admite llamadas concurrentes, así que las
    evaluaciones se serializan con un lock (cada una dura microsegundos).
    """

    def __init__(
        self,
        wasm: bytes,
        data: Optional[Mapping[str, Any]] = None,
        entrypoint: str = DEFAULT_ENTRYPOINT,
    ):
        try:
            import wasmtime
        except ImportError as exc:  # pragma: no cover - depende del entorno
            raise PolicyError(
                "el enforcement local requiere wasmtime: pip install 'agentlens[policy]'"
            ) from exc
        self._wt = wasmtime
        self._engine = wasmtime.Engine()
        try:
            self._module = wasmtime.Module(self._engine, wasm)
        except Exception as exc:
            raise PolicyError(f"policy.wasm inválido: {exc}") from exc
        self._data_json = json.dumps(dict(data or {})).encode("utf-8")
        self._entrypoint_name = entrypoint
        self._lock = threading.Lock()
        self._instantiate()

    @classmethod
    def from_bundle(
        cls,
        path: str,
        data_override: Optional[Mapping[str, Any]] = None,
        entrypoint: str = DEFAULT_ENTRYPOINT,
    ) -> "WasmPolicy":
        """Carga un bundle ``.tar.gz`` de OPA (policy.wasm + data.json).

        Se lee en memoria; no se extrae nada a disco. ``data_override`` sustituye
        claves de primer nivel de ``data`` (p.ej. ``{"config": {...}}``).
        """
        files: Dict[str, bytes] = {}
        try:
            with tarfile.open(path, "r:gz") as tar:
                for member in tar.getmembers():
                    if member.isfile():
                        fh = tar.extractfile(member)
                        if fh is not None:
                            files[member.name.lstrip("/")] = fh.read()
        except (OSError, tarfile.TarError) as exc:
            raise PolicyError(f"no se pudo leer el bundle {path}: {exc}") from exc
        if "policy.wasm" not in files:
            raise PolicyError(f"el bundle {path} no contiene policy.wasm")
        data = json.loads(files.get("data.json", b"{}") or b"{}")
        if data_override:
            data.update(data_override)
        return cls(files["policy.wasm"], data, entrypoint)

    # -- ciclo de vida de la instancia --

    def _instantiate(self) -> None:
        wt = self._wt
        store = wt.Store(self._engine)
        memory = None
        imports = []
        for imp in self._module.imports:
            if imp.module != "env":
                raise PolicyError(f"import WASM no soportado: {imp.module}.{imp.name}")
            if imp.name == "memory":
                memory = wt.Memory(store, imp.type)
                imports.append(memory)
            elif imp.name == "opa_abort":
                imports.append(wt.Func(store, imp.type, self._abort))
            elif imp.name.startswith("opa_builtin"):
                imports.append(wt.Func(store, imp.type, self._dispatch_builtin))
            elif imp.name == "opa_println":
                imports.append(wt.Func(store, imp.type, lambda addr: None))
            else:
                raise PolicyError(f"import WASM no soportado: env.{imp.name}")
        if memory is None:
            raise PolicyError("policy.wasm no importa env.memory")

        self._store = store
        self._memory = memory
        instance = wt.Instance(store, self._module, imports)
        exports = instance.exports(store)

        major = exports["opa_wasm_abi_version"].value(store)
        minor = exports["opa_wasm_abi_minor_version"].value(store)
        if major != 1 or minor < 2:
            raise PolicyError(f"ABI de OPA no soportado: {major}.{minor} (se requiere 1.2+)")

        ex: Dict[str, Callable[..., int]] = {}
        for name in ("opa_malloc", "opa_json_parse", "opa_json_dump", "opa_eval",
                     "opa_heap_ptr_get", "builtins", "entrypoints"):
            try:
                ex[name] = _i32_caller(wt, store, exports[name])
            except KeyError:
                raise PolicyError(f"policy.wasm no exporta {name}") from None
        self._ex = ex

        self._builtin_ids: Dict[int, Callable[..., Any]] = {}
        for name, bid in self._read_json(ex["opa_json_dump"](ex["builtins"]())).items():
            if name not in _BUILTINS:
                raise PolicyError(f"la política usa un builtin no soportado por el host: {name}")
            self._builtin_ids[bid] = _BUILTINS[name]

        entrypoints = self._read_json(ex["opa_json_dump"](ex["entrypoints"]()))
        if self._entrypoint_name not in entrypoints:
            raise PolicyError(
                f"entrypoint {self._entrypoint_name!r} no está en el bundle: {sorted(entrypoints)}"
            )
        self._entrypoint = entrypoints[self._entrypoint_name]

        self._data_addr = self._parse_json(self._data_json)
        self._base_heap = ex["opa_heap_ptr_get"]()

    # -- utilidades de memoria --

    def _read_cstr(self, addr: int) -> bytes:
        out = bytearray()
        size = self._memory.data_len(self._store)
        chunk = 1024
        while addr < size:
            buf = self._memory.read(self._store, addr, min(addr + chunk, size))
            nul = buf.find(0)
            if nul >= 0:
                out += buf[:nul]
                return bytes(out)
            out += buf
            addr += len(buf)
        raise PolicyError("cadena WASM sin terminador")

    def _read_json(self, addr: int) -> Any:
        return json.loads(self._read_cstr(addr))

    def _parse_json(self, raw: bytes) -> int:
        ex = self._ex
        addr = ex["opa_malloc"](len(raw))
        self._memory.write(self._store, raw, addr)
        value = ex["opa_json_parse"](addr, len(raw))
        if value == 0:
            raise PolicyError("opa_json_parse falló")
        return value

    def _ensure_capacity(self, end: int) -> None:
        size = self._memory.data_len(self._store)
        if end > size:
            self._memory.grow(self._store, (end - size + _WASM_PAGE - 1) // _WASM_PAGE)

    # -- callbacks del host --

    def _abort(self, addr: int) -> None:
        raise PolicyError("opa_abort: " + self._read_cstr(addr).decode("utf-8", "replace"))

    def _dispatch_builtin(self, builtin_id: int, _ctx: int, *arg_addrs: int) -> int:
        fn = self._builtin_ids.get(builtin_id)
        if fn is None:
            raise PolicyError(f"builtin desconocido: {builtin_id}")
        args = [self._read_json(self._ex["opa_json_dump"](a)) for a in arg_addrs]
        return self._parse_json(json.dumps(fn(*args)).encode("utf-8"))

    # -- evaluación --

    def evaluate(self, action: Mapping[str, Any]) -> PolicyDecision:
        """Decide sobre una acción. Nunca lanza: los errores deniegan (fail-closed)."""
        start = time.perf_counter()
        with self._lock:
            try:
                doc = self._eval_locked(json.dumps(dict(action)).encode("utf-8"))
            except Exception as exc:  # trap, abort, JSON inesperado...
                # La instancia puede haber quedado inconsistente tras un trap.
                try:
                    self._instantiate()
                except Exception:
                    pass
                return PolicyDecision(
                    allow=False,
                    denials=[f"error evaluando la política: {exc}"],
                    eval_us=(time.perf_counter() - start) * 1e6,
                )
        elapsed = (time.perf_counter() - start) * 1e6

        allow = doc.get("allow") is True
        denials = sorted(str(d) for d in doc.get("deny", []) or [])
        if not allow and not denials:
            denials = ["denegado por política"]
        return PolicyDecision(allow=allow, denials=denials, eval_us=elapsed)

    def _eval_locked(self, raw: bytes) -> Dict[str, Any]:
        input_addr = self._base_heap
        self._ensure_capacity(input_addr + len(raw))
        self._memory.write(self._store, raw, input_addr)
        result_addr = self._ex["opa_eval"](
            0, self._entrypoint, self._data_addr,
            input_addr, len(raw), input_addr + len(raw), 0,
        )
        result = self._read_json(result_addr)
        if not isinstance(result, list) or not result:
            return {}  # sin resultado => no hay allow => deniega
        doc = result[0].get("result")
        if not isinstance(doc, dict):
            raise PolicyError(f"resultado inesperado de la política: {doc!r}")
        return doc
