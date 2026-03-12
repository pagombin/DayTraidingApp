"""Trading mode configuration step."""

import questionary
from rich.console import Console
from rich.panel import Panel

console = Console()


def run(config: dict) -> dict:
    """Ask whether to use paper or live trading."""
    console.print(
        Panel(
            "[bold yellow]Paper trading is strongly recommended[/bold yellow]\n\n"
            "Paper trading lets you test strategies with simulated money.\n"
            "Live trading uses real funds and carries real financial risk.",
            border_style="yellow",
            padding=(1, 2),
        )
    )

    mode = questionary.select(
        "Select trading mode:",
        choices=["Paper Trading (recommended)", "Live Trading"],
    ).ask()

    if mode is None:
        raise KeyboardInterrupt

    paper_mode = True

    if mode == "Live Trading":
        console.print(
            "\n[bold red]WARNING: Live trading uses real money. "
            "You could lose your entire investment.[/bold red]\n"
        )
        confirmation = questionary.text(
            'To confirm live trading, type exactly "I ACCEPT THE RISK":',
        ).ask()

        if confirmation is None:
            raise KeyboardInterrupt

        if confirmation == "I ACCEPT THE RISK":
            paper_mode = False
            console.print("[red]Live trading enabled.[/red]")
        else:
            paper_mode = True
            console.print("[yellow]Confirmation did not match. Defaulting to paper trading.[/yellow]")

    if paper_mode:
        console.print("[green]Paper trading enabled.[/green]")

    return {
        "trading": {
            "paper_mode": paper_mode,
        }
    }
