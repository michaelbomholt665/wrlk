#!/usr/bin/env python3
"""Export source files to Markdown.

This script can either:
1. Combine matching source files into a single Markdown file.
2. Create one Markdown file per source file in an output directory.

Supported defaults:
- .go
- .py
- .ts

Examples:
    python export_to_markdown.py \
        --input ./app ./lib \
        --mode combined \
        --output ./exports/code_dump.md

    python export_to_markdown.py \
        --input ./app ./lib \
        --mode split \
        --output-dir ./exports/per_file_md

    python export_to_markdown.py \
        --input ./src \
        --extensions .py .ts \
        --exclude .venv node_modules dist \
        --mode combined \
        --output ./exports/source_bundle.md
"""

# pyright: reportUnusedCallResult=false
from __future__ import annotations

import argparse
import sys
from collections.abc import Iterable
from dataclasses import dataclass
from pathlib import Path
from typing import Literal, cast


DEFAULT_EXTENSIONS: tuple[str, ...] = (".go", ".py", ".ts")
DEFAULT_EXCLUDES: tuple[str, ...] = (
    ".git",
    ".idea",
    ".vscode",
    "__pycache__",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    ".venv",
    "venv",
    "node_modules",
    "dist",
    "build",
    "coverage",
)

LANGUAGE_MAP: dict[str, str] = {
    ".go": "go",
    ".py": "python",
    ".ts": "ts",
}

ExportMode = Literal["combined", "split"]


@dataclass(frozen=True)
class SourceFile:
    """Represents a source file discovered for export."""

    root: Path
    path: Path

    @property
    def relative_path(self) -> Path:
        """Return the source path relative to its declared root."""
        return self.path.relative_to(self.root)


@dataclass(frozen=True)
class CliArgs:
    """Typed CLI arguments."""

    input_paths: list[str]
    extensions: list[str]
    exclude: list[str]
    mode: ExportMode
    output: str | None
    output_dir: str | None
    encoding: str
    include_hidden: bool


def build_parser() -> argparse.ArgumentParser:
    """Build the CLI argument parser."""
    parser = argparse.ArgumentParser(description="Export selected source files to Markdown.")

    _ = parser.add_argument(
        "--input",
        nargs="+",
        required=True,
        help="One or more input files or directories.",
    )
    _ = parser.add_argument(
        "--extensions",
        nargs="+",
        default=list(DEFAULT_EXTENSIONS),
        help="File extensions to include. Example: .py .go .ts",
    )
    _ = parser.add_argument(
        "--exclude",
        nargs="*",
        default=list(DEFAULT_EXCLUDES),
        help="Directory names to exclude during recursive scanning.",
    )
    _ = parser.add_argument(
        "--mode",
        choices=("combined", "split"),
        required=True,
        help=("Export mode: combined = one Markdown file, split = one Markdown file per source file."),
    )
    _ = parser.add_argument(
        "--output",
        help="Path to combined Markdown file. Required when mode=combined.",
    )
    _ = parser.add_argument(
        "--output-dir",
        help="Directory for per-file Markdown output. Required when mode=split.",
    )
    _ = parser.add_argument(
        "--encoding",
        default="utf-8",
        help="File encoding used for reading source files. Default: utf-8",
    )
    _ = parser.add_argument(
        "--include-hidden",
        action="store_true",
        help="Include hidden files and directories.",
    )

    return parser


def parse_args() -> CliArgs:
    """Parse and validate CLI arguments into a typed object."""
    parser = build_parser()
    namespace = parser.parse_args()

    input_paths = cast(list[str], namespace.input)
    extensions = cast(list[str], namespace.extensions)
    exclude = cast(list[str], namespace.exclude)
    mode = cast(ExportMode, namespace.mode)
    output = cast(str | None, namespace.output)
    output_dir = cast(str | None, namespace.output_dir)
    encoding = cast(str, namespace.encoding)
    include_hidden = cast(bool, namespace.include_hidden)

    args = CliArgs(
        input_paths=input_paths,
        extensions=extensions,
        exclude=exclude,
        mode=mode,
        output=output,
        output_dir=output_dir,
        encoding=encoding,
        include_hidden=include_hidden,
    )
    validate_args(args)
    return args


def normalize_extensions(extensions: Iterable[str]) -> tuple[str, ...]:
    """Normalize extensions to lowercase and ensure a leading dot."""
    normalized: list[str] = []

    for ext in extensions:
        clean_ext = ext.strip().lower()
        if not clean_ext:
            continue
        if not clean_ext.startswith("."):
            clean_ext = f".{clean_ext}"
        normalized.append(clean_ext)

    return tuple(sorted(set(normalized)))


def should_skip_name(
    name: str,
    excluded_names: set[str],
    include_hidden: bool,
) -> bool:
    """Return True when a file or directory name should be skipped."""
    if name in excluded_names:
        return True
    if not include_hidden and name.startswith("."):
        return True
    return False


def should_skip_relative_path(
    relative_path: Path,
    excluded_names: set[str],
    include_hidden: bool,
) -> bool:
    """Return True if any component in a relative path should be skipped."""
    for part in relative_path.parts:
        if should_skip_name(
            name=part,
            excluded_names=excluded_names,
            include_hidden=include_hidden,
        ):
            return True
    return False


def read_text_file(file_path: Path, encoding: str) -> str:
    """Read a text file safely."""
    try:
        return file_path.read_text(encoding=encoding)
    except UnicodeDecodeError:
        return file_path.read_text(encoding=encoding, errors="replace")


def collect_file_from_path(
    file_path: Path,
    root: Path,
    extensions: tuple[str, ...],
    excluded_names: set[str],
    include_hidden: bool,
) -> SourceFile | None:
    """Create a SourceFile from a path when it matches export rules."""
    if file_path.suffix.lower() not in extensions:
        return None

    relative_path = file_path.relative_to(root)
    if should_skip_relative_path(
        relative_path=relative_path,
        excluded_names=excluded_names,
        include_hidden=include_hidden,
    ):
        return None

    return SourceFile(root=root, path=file_path)


def collect_from_file_input(
    input_path: Path,
    extensions: tuple[str, ...],
) -> SourceFile | None:
    """Handle a direct file input."""
    if input_path.suffix.lower() not in extensions:
        return None
    return SourceFile(root=input_path.parent, path=input_path)


def collect_from_directory_input(
    root: Path,
    extensions: tuple[str, ...],
    excluded_names: set[str],
    include_hidden: bool,
) -> list[SourceFile]:
    """Collect matching files from a directory input."""
    collected: list[SourceFile] = []

    for path in root.rglob("*"):
        if path.is_dir():
            continue

        source_file = collect_file_from_path(
            file_path=path,
            root=root,
            extensions=extensions,
            excluded_names=excluded_names,
            include_hidden=include_hidden,
        )
        if source_file is not None:
            collected.append(source_file)

    return collected


def collect_source_files(
    inputs: list[str],
    extensions: tuple[str, ...],
    excluded_names: set[str],
    include_hidden: bool,
) -> list[SourceFile]:
    """Collect source files from the provided input paths."""
    results: list[SourceFile] = []

    for raw_input in inputs:
        input_path = Path(raw_input).resolve()

        if not input_path.exists():
            print(f"[WARN] Skipping missing path: {input_path}", file=sys.stderr)
            continue

        if input_path.is_file():
            source_file = collect_from_file_input(
                input_path=input_path,
                extensions=extensions,
            )
            if source_file is not None:
                results.append(source_file)
            continue

        results.extend(
            collect_from_directory_input(
                root=input_path,
                extensions=extensions,
                excluded_names=excluded_names,
                include_hidden=include_hidden,
            )
        )

    unique_sorted = sorted(
        set(results),
        key=lambda item: (str(item.root).lower(), str(item.relative_path).lower()),
    )
    return unique_sorted


def markdown_code_block(file_path: Path, content: str) -> str:
    """Render a Markdown section for a single source file."""
    language = LANGUAGE_MAP.get(file_path.suffix.lower(), "")
    separator = "\n---\n"

    return f"{separator}\n## File: `{file_path.as_posix()}`\n\n```{language}\n{content.rstrip()}\n```\n"


def write_combined_markdown(
    source_files: list[SourceFile],
    output_file: Path,
    encoding: str,
) -> None:
    """Write all collected source files into one Markdown file."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    lines: list[str] = [
        "# Source Export",
        "",
        f"Total files: {len(source_files)}",
        "",
    ]

    for source in source_files:
        content = read_text_file(source.path, encoding)
        lines.append(markdown_code_block(source.relative_path, content))

    output_file.write_text("\n".join(lines).rstrip() + "\n", encoding=encoding)


def safe_markdown_name(relative_path: Path) -> Path:
    """Create a safe Markdown output filename from a relative source path."""
    parts = tuple(relative_path.parts)
    if not parts:
        return Path("unknown.md")

    filename = parts[-1]
    stem = Path(filename).stem
    suffix = Path(filename).suffix.lstrip(".")

    folder_parts_list: list[str] = []
    if len(parts) > 1:
        for index, part in enumerate(parts):
            if index == len(parts) - 1:
                break
            folder_parts_list.append(part)

    if folder_parts_list:
        flattened_prefix = "__".join(folder_parts_list)
        out_name = f"{flattened_prefix}__{stem}.{suffix}.md"
    else:
        out_name = f"{stem}.{suffix}.md"

    return Path(out_name)


def write_split_markdown(
    source_files: list[SourceFile],
    output_dir: Path,
    encoding: str,
) -> None:
    """Write one Markdown file per collected source file."""
    output_dir.mkdir(parents=True, exist_ok=True)

    for source in source_files:
        content = read_text_file(source.path, encoding)
        md_name = safe_markdown_name(source.relative_path)
        output_path = output_dir / md_name

        markdown = (
            f"# File: `{source.relative_path.as_posix()}`\n\n"
            f"Source root: `{source.root.as_posix()}`\n\n"
            f"```{LANGUAGE_MAP.get(source.path.suffix.lower(), '')}\n"
            f"{content.rstrip()}\n"
            f"```\n"
        )
        output_path.write_text(markdown, encoding=encoding)


def validate_args(args: CliArgs) -> None:
    """Validate argument combinations."""
    if args.mode == "combined" and not args.output:
        raise ValueError("--output is required when --mode combined is used.")
    if args.mode == "split" and not args.output_dir:
        raise ValueError("--output-dir is required when --mode split is used.")


def main() -> int:
    """Program entry point."""
    try:
        args = parse_args()
        extensions = normalize_extensions(args.extensions)
        excluded_names = set(args.exclude)

        source_files = collect_source_files(
            inputs=args.input_paths,
            extensions=extensions,
            excluded_names=excluded_names,
            include_hidden=args.include_hidden,
        )

        if not source_files:
            print("[INFO] No matching files found.")
            return 0

        if args.mode == "combined":
            output_value = args.output
            if output_value is None:
                raise ValueError("Missing output path for combined mode.")

            output_file = Path(output_value).resolve()
            write_combined_markdown(
                source_files=source_files,
                output_file=output_file,
                encoding=args.encoding,
            )
            print(f"[OK] Wrote combined Markdown: {output_file}")
            print(f"[OK] Exported files: {len(source_files)}")
            return 0

        output_dir_value = args.output_dir
        if output_dir_value is None:
            raise ValueError("Missing output directory for split mode.")

        output_dir = Path(output_dir_value).resolve()
        write_split_markdown(
            source_files=source_files,
            output_dir=output_dir,
            encoding=args.encoding,
        )
        print(f"[OK] Wrote per-file Markdown to: {output_dir}")
        print(f"[OK] Exported files: {len(source_files)}")
        return 0

    except ValueError as exc:
        print(f"[ERROR] {exc}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        print("\n[ERROR] Interrupted.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
