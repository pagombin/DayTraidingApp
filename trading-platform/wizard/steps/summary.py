"""Summary step - displays all chosen config and asks for confirmation."""

import questionary
from rich.console import Console
from rich.table import Table

console = Console()


def run(config: dict) -> bool:
    """Show a summary table of all config and ask to proceed."""
    table = Table(title="Configuration Summary", show_lines=True)
    table.add_column("Setting", style="bold cyan")
    table.add_column("Value", style="white")

    # Broker
    broker = config.get("broker", {})
    table.add_row("Broker", broker.get("name", "N/A").upper())
    table.add_row("Broker API Key", _mask(broker.get("api_key", "")))

    # Data Provider
    data = config.get("data_provider", {})
    table.add_row("Data Provider", data.get("name", "N/A").upper())
    table.add_row("Data Provider API Key", _mask(data.get("api_key", "")))

    # Trading Mode
    trading = config.get("trading", {})
    mode_str = "Paper" if trading.get("paper_mode", True) else "LIVE"
    table.add_row("Trading Mode", mode_str)

    # Risk
    risk = config.get("risk", {})
    table.add_row("Risk Preset", risk.get("preset", "N/A"))
    table.add_row("Max Daily Loss", f"${risk.get('max_daily_loss', 'N/A')}")
    table.add_row("Max Position %", f"{risk.get('max_position_pct', 'N/A')}%")
    table.add_row("Require Stop Loss", str(risk.get("require_stop_loss", "N/A")))
    table.add_row("Max Orders/Min", str(risk.get("max_orders_per_minute", "N/A")))

    # Watchlist
    wl = config.get("watchlist", {})
    symbols = wl.get("symbols", [])
    table.add_row("Watchlist", f"{wl.get('preset', 'N/A')} ({len(symbols)} symbols)")
    table.add_row("Symbols", ", ".join(symbols) if symbols else "N/A")

    # Notifications
    notif = config.get("notifications", {})
    channels = []
    if notif.get("email"):
        channels.append("Email")
    if notif.get("sms_phone"):
        channels.append("SMS")
    if notif.get("slack_webhook"):
        channels.append("Slack")
    if notif.get("discord_webhook"):
        channels.append("Discord")
    if notif.get("telegram_bot_token"):
        channels.append("Telegram")
    table.add_row("Notifications", ", ".join(channels) if channels else "None")

    # Prerequisites
    prereqs = config.get("prerequisites", {})
    docker_status = "Yes" if prereqs.get("docker_installed") else "No"
    compose_status = "Yes" if prereqs.get("docker_compose_installed") else "No"
    table.add_row("Docker Installed", docker_status)
    table.add_row("Docker Compose Installed", compose_status)

    console.print()
    console.print(table)
    console.print()

    proceed = questionary.confirm(
        "Proceed with these settings?",
        default=True,
    ).ask()

    if proceed is None:
        raise KeyboardInterrupt

    return proceed


def _mask(value: str) -> str:
    """Mask a sensitive value, showing only the last 4 characters."""
    if not value or len(value) <= 4:
        return "****"
    return "*" * (len(value) - 4) + value[-4:]
