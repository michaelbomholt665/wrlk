#!/usr/bin/env python3
# personas.py
"""
Persona compiler for agent system prompt generation.

Resolves flag-based persona selection into a composed JSON payload
containing a merged system prompt and structured config block.
Enforces a hard cap of 2 matched personas per invocation.
"""

################
#   IMPORTS
################

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass


################
#   CONSTANTS
################

MAX_PERSONA_MERGE = 2


################
#   CLASSES
################


@dataclass(frozen=True, slots=True)
class PersonaDefinition:
    """
    Immutable persona profile resolved at compile time.

    Carries the identity, routing metadata, and instruction set
    for a single agent persona. Designed to be merged with at most
    one other persona before emission.
    """

    key: str
    summary: str
    aliases: tuple[str, ...]
    trigger_keywords: tuple[str, ...]
    instructions: tuple[str, ...]


################
#   DATA
################

PERSONAS: dict[str, PersonaDefinition] = {
    "architect": PersonaDefinition(
        key="architect",
        summary="Bias toward decomposition, contracts, boundaries, and long-term maintainability.",
        aliases=("architect", "system-architect", "design-lead"),
        trigger_keywords=("hexagonal", "architecture", "boundaries", "ports", "adapters"),
        instructions=(
            "Prefer explicit contracts, boundaries, and separation of concerns.",
            "Do not invent files casually; create files only when they serve a named responsibility.",
            "Keep folder fan-out controlled and group files by concern.",
        ),
    ),
    "implementer": PersonaDefinition(
        key="implementer",
        summary="Bias toward concrete file outputs, direct fixes, and execution detail.",
        aliases=("implementer", "coder", "builder"),
        trigger_keywords=("implement", "fix", "write code", "refactor", "patch"),
        instructions=(
            "Return full-file content when making code changes, not diff-only output.",
            "State what was validated and what was not validated.",
            "Keep solutions concrete and directly executable.",
        ),
    ),
    "security-reviewer": PersonaDefinition(
        key="security-reviewer",
        summary="Bias toward validation, least privilege, and adversarial review.",
        aliases=("security", "security-reviewer", "reviewer"),
        trigger_keywords=("security", "audit", "threat", "secret", "vulnerability"),
        instructions=(
            "Prefer least privilege, validation at boundaries, and safe defaults.",
            "Flag assumptions that could cause unsafe file or shell behavior.",
            "Do not normalize risky shortcuts merely because they are convenient.",
        ),
    ),
    "go-backend-engineer": PersonaDefinition(
        key="go-backend-engineer",
        summary="Bias toward idiomatic Go, stdlib-first solutions, and explicit request routing with minimal dependency surface.",
        aliases=("go-engineer", "go-backend", "gateway-engineer"),
        trigger_keywords=("router", "gateway", "middleware", "handler", "api", "http", "mux", "dispatch", "proxy"),
        instructions=(
            "Prefer net/http, context, and encoding/json over third-party HTTP frameworks unless the gap is substantial and named.",
            "Design handlers with explicit method dispatch; avoid magic routing patterns that obscure control flow.",
            "Gateway logic must be traceable: each request path should be followable without jumping into framework internals.",
            "Reject library additions that replace a small amount of boilerplate with a large transitive dependency tree.",
            "Middleware must be composable as plain functions or http.Handler chains, not framework-specific constructs.",
            "State what stdlib limitation, if any, justified each non-stdlib import.",
        ),
    ),
}


################
#   FUNCTIONS
################


def _build_alias_index(personas: dict[str, PersonaDefinition]) -> dict[str, str]:
    """
    Build a flat alias-to-key lookup from the persona registry.

    Maps every alias and the key itself to its canonical persona key,
    enabling flag resolution regardless of which alias was supplied.
    """
    index: dict[str, str] = {}

    for key, persona in personas.items():
        index[key] = key
        for alias in persona.aliases:
            index[alias] = key

    return index


def _resolve_flags(flags: list[str], alias_index: dict[str, str]) -> list[str]:
    """
    Resolve CLI flags to canonical persona keys.

    Strips leading dashes from each flag and looks it up in the alias
    index. Unrecognised flags are silently skipped; the caller is
    responsible for checking whether any keys were resolved at all.
    """
    resolved: list[str] = []

    for flag in flags:
        normalised = flag.lstrip("-").lower()
        canonical = alias_index.get(normalised)
        if canonical and canonical not in resolved:
            resolved.append(canonical)

    return resolved


def _merge_personas(keys: list[str], personas: dict[str, PersonaDefinition]) -> dict[str, object]:
    """
    Merge up to MAX_PERSONA_MERGE personas into a single output payload.

    Combines summaries and deduplicates instructions across matched
    personas. Returns a structured dict ready for JSON serialisation.
    Emits a warning to stderr if more than MAX_PERSONA_MERGE keys matched.
    """
    if len(keys) > MAX_PERSONA_MERGE:
        excess: list[str] = list(keys[MAX_PERSONA_MERGE:])
        print(
            f"[personas] warning: {len(keys)} personas matched; capped at {MAX_PERSONA_MERGE}. dropped: {excess}",
            file=sys.stderr,
        )
        keys = list(keys[:MAX_PERSONA_MERGE])

    matched = [personas[k] for k in keys]

    merged_instructions: list[str] = []
    seen: set[str] = set()

    for persona in matched:
        for instruction in persona.instructions:
            if instruction not in seen:
                merged_instructions.append(instruction)
                seen.add(instruction)

    return {
        "personas": [p.key for p in matched],
        "summary": " | ".join(p.summary for p in matched),
        "instructions": merged_instructions,
    }


def _build_parser() -> argparse.ArgumentParser:
    """
    Build the argument parser for persona flag resolution.

    Accepts any number of positional flag arguments matching persona
    keys or aliases. Unknown flags produce a warning rather than an error
    to allow loose integration with external hook systems.
    """
    parser = argparse.ArgumentParser(
        prog="personas",
        description="Resolve persona flags into a composed JSON agent config.",
        add_help=True,
    )
    _ = parser.add_argument(
        "flags",
        nargs="+",
        metavar="PERSONA_FLAG",
        help="One or more persona keys or aliases (e.g. go-backend architect).",
    )
    return parser


def _run(argv: list[str] | None = None) -> None:
    """
    Entry point for CLI invocation.

    Parses argv, resolves persona flags, merges matched definitions,
    and writes the resulting JSON payload to stdout. Exits with code 1
    if no flags resolve to a known persona.
    """
    parser = _build_parser()
    args = parser.parse_args(argv)

    alias_index = _build_alias_index(PERSONAS)
    raw_flags: list[str] = list(args.flags)
    resolved_keys = _resolve_flags(raw_flags, alias_index)

    if not resolved_keys:
        print(
            f"[personas] error: none of {raw_flags} matched a known persona or alias.",
            file=sys.stderr,
        )
        sys.exit(1)

    payload: dict[str, object] = _merge_personas(resolved_keys, PERSONAS)
    print(json.dumps(payload, indent=2))


################
#   ENTRY
################

if __name__ == "__main__":
    _run()
