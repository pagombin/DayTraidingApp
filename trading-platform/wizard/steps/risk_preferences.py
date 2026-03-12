"""Risk preferences configuration step."""

import questionary
from rich.console import Console
from rich.table import Table

console = Console()

PRESETS = {
    "Conservative": {
        "max_daily_loss": 200,
        "max_position_pct": 3,
        "require_stop_loss": True,
        "max_orders_per_minute": 5,
    },
    "Moderate": {
        "max_daily_loss": 500,
        "max_position_pct": 5,
        "require_stop_loss": True,
        "max_orders_per_minute": 10,
    },
    "Aggressive": {
        "max_daily_loss": 1000,
        "max_position_pct": 10,
        "require_stop_loss": False,
        "max_orders_per_minute": 30,
    },
}


def run(config: dict) -> dict:
    """Show risk presets and let the user choose one."""
    table = Table(title="Risk Presets")
    table.add_column("Preset", style="bold")
    table.add_column("Max Daily Loss", justify="right")
    table.add_column("Max Position %", justify="right")
    table.add_column("Require Stop Loss")
    table.add_column("Max Orders/Min", justify="right")

    for name, params in PRESETS.items():
        table.add_row(
            name,
            f"${params['max_daily_loss']}",
            f"{params['max_position_pct']}%",
            str(params["require_stop_loss"]),
            str(params["max_orders_per_minute"]),
        )

    console.print(table)
    console.print()

    choice = questionary.select(
        "Select a risk preset:",
        choices=list(PRESETS.keys()),
    ).ask()

    if choice is None:
        raise KeyboardInterrupt

    selected = PRESETS[choice]
    console.print(f"[green]Risk preset selected:[/green] {choice}")

    return {
        "risk": {
            "preset": choice,
            **selected,
        }
    }
