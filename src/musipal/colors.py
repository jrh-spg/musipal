from __future__ import annotations

from colored import attr, fg


def c(text: str, color: str | None = None, *, bold: bool = False) -> str:
    if not color:
        return text
    style = ""
    if bold:
        style += attr("bold")
    return f"{style}{fg(color)}{text}{attr('reset')}"


def status_ok(text: str) -> str:
    return c(text, "green", bold=True)


def status_warn(text: str) -> str:
    return c(text, "yellow", bold=True)


def status_err(text: str) -> str:
    return c(text, "red", bold=True)
