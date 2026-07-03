"""AgentLens Compliance — mapeo regulatorio traza → requisito (E5)."""
from .evaluator import Report, RequirementResult, evaluate
from .evidence import Facts, Span
from .model import Mapping, MappedRequirement, Requirement, Status

__version__ = "0.1.0"

__all__ = [
    "evaluate",
    "Report",
    "RequirementResult",
    "Facts",
    "Span",
    "Mapping",
    "MappedRequirement",
    "Requirement",
    "Status",
    "__version__",
]
