"""Broker configuration step."""

import questionary
from rich.console import Console

console = Console()

BROKERS = ["alpaca", "ibkr", "tradier", "schwab"]


def run(config: dict) -> dict:
    """Ask which broker to use and collect API credentials."""
    broker_name = questionary.select(
        "Which broker would you like to use?",
        choices=BROKERS,
    ).ask()

    if broker_name is None:
        raise KeyboardInterrupt

    api_key = questionary.text(
        f"Enter your {broker_name.upper()} API Key:",
    ).ask()

    if api_key is None:
        raise KeyboardInterrupt

    api_secret = questionary.password(
        f"Enter your {broker_name.upper()} API Secret:",
    ).ask()

    if api_secret is None:
        raise KeyboardInterrupt

    console.print(f"[green]Broker configured:[/green] {broker_name.upper()}")

    return {
        "broker": {
            "name": broker_name,
            "api_key": api_key,
            "api_secret": api_secret,
        }
    }
