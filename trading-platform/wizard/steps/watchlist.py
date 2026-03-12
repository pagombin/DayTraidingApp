"""Watchlist configuration step."""

import questionary
from rich.console import Console

console = Console()

PRESET_LISTS = {
    "Top Tech": ["AAPL", "MSFT", "NVDA", "GOOG", "META", "AMZN", "TSLA"],
    "S&P 500 Top 20": [
        "SPY", "QQQ", "AAPL", "MSFT", "NVDA", "AMZN", "GOOG",
        "TSLA", "META", "JPM", "V", "UNH", "XOM", "MA", "HD",
        "PG", "JNJ", "COST", "ABBV", "MRK",
    ],
    "Semiconductor": [
        "NVDA", "AMD", "AVGO", "QCOM", "INTC", "TSM", "MU",
        "MRVL", "LRCX", "AMAT",
    ],
    "Energy": [
        "XOM", "CVX", "COP", "SLB", "MPC", "EOG", "PSX",
        "VLO", "OXY", "HAL",
    ],
    "Custom": [],
}


def run(config: dict) -> dict:
    """Let the user pick a preset watchlist or enter custom symbols."""
    choice = questionary.select(
        "Select a watchlist:",
        choices=list(PRESET_LISTS.keys()),
    ).ask()

    if choice is None:
        raise KeyboardInterrupt

    if choice == "Custom":
        symbols_input = questionary.text(
            "Enter symbols separated by commas (e.g. AAPL, MSFT, GOOG):",
        ).ask()

        if symbols_input is None:
            raise KeyboardInterrupt

        symbols = [s.strip().upper() for s in symbols_input.split(",") if s.strip()]
    else:
        symbols = PRESET_LISTS[choice]

    console.print(f"[green]Watchlist ({choice}):[/green] {', '.join(symbols)}")

    return {
        "watchlist": {
            "preset": choice,
            "symbols": symbols,
        }
    }
