"""Welcome step - displays platform info."""

from rich.console import Console
from rich.panel import Panel

console = Console()


def run(config: dict) -> None:
    """Show a welcome panel with the platform name and description."""
    console.print(
        Panel(
            "[bold cyan]Day Trading Platform[/bold cyan]\n\n"
            "An automated trading system supporting multiple brokers,\n"
            "real-time data feeds, configurable risk management,\n"
            "and customizable trading strategies.\n\n"
            "[dim]This wizard will help you configure:[/dim]\n"
            "  - Broker and data provider connections\n"
            "  - Trading mode (paper / live)\n"
            "  - Risk management preferences\n"
            "  - Watchlist symbols\n"
            "  - Notification channels\n"
            "  - Trading strategies",
            title="[bold green]Welcome[/bold green]",
            border_style="bright_blue",
            padding=(1, 4),
        )
    )
    return None
