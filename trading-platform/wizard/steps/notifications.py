"""Notifications configuration step."""

import questionary
from rich.console import Console

console = Console()


def run(config: dict) -> dict:
    """Collect optional notification channel settings."""
    console.print("[dim]All notification fields are optional. Press Enter to skip.[/dim]\n")

    email = questionary.text(
        "Email address for notifications (optional):",
    ).ask()

    if email is None:
        raise KeyboardInterrupt

    sms_phone = questionary.text(
        "SMS phone number for notifications (optional):",
    ).ask()

    if sms_phone is None:
        raise KeyboardInterrupt

    slack_webhook = questionary.text(
        "Slack webhook URL (optional):",
    ).ask()

    if slack_webhook is None:
        raise KeyboardInterrupt

    discord_webhook = questionary.text(
        "Discord webhook URL (optional):",
    ).ask()

    if discord_webhook is None:
        raise KeyboardInterrupt

    telegram_token = questionary.text(
        "Telegram bot token (optional):",
    ).ask()

    if telegram_token is None:
        raise KeyboardInterrupt

    telegram_chat_id = ""
    if telegram_token:
        telegram_chat_id = questionary.text(
            "Telegram chat ID:",
        ).ask()

        if telegram_chat_id is None:
            raise KeyboardInterrupt

    # Summarize what was configured
    channels = []
    if email:
        channels.append("Email")
    if sms_phone:
        channels.append("SMS")
    if slack_webhook:
        channels.append("Slack")
    if discord_webhook:
        channels.append("Discord")
    if telegram_token:
        channels.append("Telegram")

    if channels:
        console.print(f"[green]Notifications configured:[/green] {', '.join(channels)}")
    else:
        console.print("[yellow]No notification channels configured.[/yellow]")

    return {
        "notifications": {
            "email": email or "",
            "sms_phone": sms_phone or "",
            "slack_webhook": slack_webhook or "",
            "discord_webhook": discord_webhook or "",
            "telegram_bot_token": telegram_token or "",
            "telegram_chat_id": telegram_chat_id or "",
        }
    }
