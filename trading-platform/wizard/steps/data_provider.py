"""Data provider configuration step."""

import questionary
from rich.console import Console

console = Console()

DATA_PROVIDERS = ["alpaca", "polygon", "tradier", "alpha_vantage"]


def run(config: dict) -> dict:
    """Ask which data provider to use, optionally reusing broker API key."""
    provider = questionary.select(
        "Which data provider would you like to use?",
        choices=DATA_PROVIDERS,
    ).ask()

    if provider is None:
        raise KeyboardInterrupt

    broker_config = config.get("broker", {})
    broker_name = broker_config.get("name")

    api_key = None
    api_secret = None

    if provider == broker_name and broker_name is not None:
        reuse = questionary.confirm(
            f"Reuse your {broker_name.upper()} API key for the data provider?",
            default=True,
        ).ask()

        if reuse is None:
            raise KeyboardInterrupt

        if reuse:
            api_key = broker_config.get("api_key", "")
            api_secret = broker_config.get("api_secret", "")
            console.print("[green]Reusing broker API credentials for data provider.[/green]")

    if api_key is None:
        api_key = questionary.text(
            f"Enter your {provider.upper()} data provider API Key:",
        ).ask()

        if api_key is None:
            raise KeyboardInterrupt

        api_secret = questionary.password(
            f"Enter your {provider.upper()} data provider API Secret (leave blank if none):",
        ).ask()

        if api_secret is None:
            raise KeyboardInterrupt

    console.print(f"[green]Data provider configured:[/green] {provider.upper()}")

    return {
        "data_provider": {
            "name": provider,
            "api_key": api_key,
            "api_secret": api_secret,
        }
    }
