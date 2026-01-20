from __future__ import annotations

import argparse

from musipal.app import run_app


def main() -> None:
    parser = argparse.ArgumentParser(prog="musipal", description="CLI/TUI music player")
    parser.add_argument(
        "--library-root",
        default=None,
        help="Override library root path (defaults to config)",
    )
    args = parser.parse_args()

    run_app(library_root_override=args.library_root)


if __name__ == "__main__":
    main()
