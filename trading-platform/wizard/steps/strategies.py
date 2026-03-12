"""Strategies configuration step - placeholder for Module 5."""

from rich.console import Console
from rich.panel import Panel

console = Console()


def run(config: dict) -> dict:
    """Show placeholder message for strategy configuration."""
    console.print(
        Panel(
            "[bold cyan]Strategy configuration coming in Module 5[/bold cyan]\n\n"
            "[dim]Default strategies will be used until then.[/dim]",
            border_style="cyan",
            padding=(1, 2),
        )
    )
    return {}
