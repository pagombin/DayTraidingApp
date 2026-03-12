#!/usr/bin/env python3
"""Interactive setup wizard for the Day Trading Platform."""

import sys

from rich.console import Console
from rich.panel import Panel

from steps import (
    welcome,
    prerequisites,
    broker,
    data_provider,
    trading_mode,
    risk_preferences,
    watchlist,
    notifications,
    strategies,
    summary,
)
from utils.config_writer import write_config

console = Console()


def main():
    """Run the setup wizard, collecting config from each step in sequence."""
    config = {}

    console.print(
        Panel(
            "[bold cyan]Day Trading Platform[/bold cyan]\n\n"
            "Welcome to the interactive setup wizard.\n"
            "This wizard will guide you through configuring your\n"
            "trading platform step by step.",
            title="[bold green]Setup Wizard[/bold green]",
            border_style="bright_blue",
            padding=(1, 4),
        )
    )

    steps = [
        ("Welcome", welcome),
        ("Prerequisites Check", prerequisites),
        ("Broker Configuration", broker),
        ("Data Provider", data_provider),
        ("Trading Mode", trading_mode),
        ("Risk Preferences", risk_preferences),
        ("Watchlist", watchlist),
        ("Notifications", notifications),
        ("Strategies", strategies),
        ("Summary", summary),
    ]

    for step_name, step_module in steps:
        console.print()
        console.rule(f"[bold yellow]{step_name}[/bold yellow]")
        console.print()

        result = step_module.run(config)

        if step_name == "Summary":
            if not result:
                console.print("[bold red]Setup cancelled by user.[/bold red]")
                sys.exit(1)
        elif result is not None:
            config.update(result)

    # Write the final configuration
    console.print()
    console.rule("[bold green]Writing Configuration[/bold green]")
    env_path = write_config(config)
    console.print(f"\n[bold green]Configuration written to:[/bold green] {env_path}")
    console.print(
        Panel(
            "[bold green]Setup complete![/bold green]\n\n"
            "You can now start the platform with:\n"
            "  [cyan]docker compose up -d[/cyan]",
            border_style="green",
            padding=(1, 4),
        )
    )


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        console.print("\n\n[bold yellow]Setup wizard interrupted. No changes were made.[/bold yellow]")
        sys.exit(130)
