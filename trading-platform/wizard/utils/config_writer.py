"""Utility to write the collected configuration to a .env file."""

import os
import secrets
from pathlib import Path


def write_config(config: dict) -> str:
    """Generate a .env file in the project root from the collected config dict.

    Args:
        config: The accumulated configuration dictionary from all wizard steps.

    Returns:
        The absolute path to the generated .env file.
    """
    # Project root is two levels up from the wizard directory
    project_root = Path(__file__).resolve().parent.parent.parent
    env_path = project_root / ".env"

    jwt_secret = secrets.token_hex(32)

    lines = []
    lines.append("# ============================================")
    lines.append("# Day Trading Platform - Generated Configuration")
    lines.append("# ============================================")
    lines.append("")

    # JWT Secret
    lines.append("# Authentication")
    lines.append(f"JWT_SECRET={jwt_secret}")
    lines.append("")

    # Broker
    broker = config.get("broker", {})
    lines.append("# Broker Configuration")
    lines.append(f"BROKER_NAME={broker.get('name', '')}")
    lines.append(f"BROKER_API_KEY={broker.get('api_key', '')}")
    lines.append(f"BROKER_API_SECRET={broker.get('api_secret', '')}")
    lines.append("")

    # Data Provider
    data = config.get("data_provider", {})
    lines.append("# Data Provider Configuration")
    lines.append(f"DATA_PROVIDER_NAME={data.get('name', '')}")
    lines.append(f"DATA_PROVIDER_API_KEY={data.get('api_key', '')}")
    lines.append(f"DATA_PROVIDER_API_SECRET={data.get('api_secret', '')}")
    lines.append("")

    # Trading Mode
    trading = config.get("trading", {})
    lines.append("# Trading Mode")
    lines.append(f"PAPER_MODE={'true' if trading.get('paper_mode', True) else 'false'}")
    lines.append("")

    # Risk Preferences
    risk = config.get("risk", {})
    lines.append("# Risk Management")
    lines.append(f"RISK_PRESET={risk.get('preset', '')}")
    lines.append(f"MAX_DAILY_LOSS={risk.get('max_daily_loss', '')}")
    lines.append(f"MAX_POSITION_PCT={risk.get('max_position_pct', '')}")
    lines.append(f"REQUIRE_STOP_LOSS={'true' if risk.get('require_stop_loss') else 'false'}")
    lines.append(f"MAX_ORDERS_PER_MINUTE={risk.get('max_orders_per_minute', '')}")
    lines.append("")

    # Watchlist
    wl = config.get("watchlist", {})
    symbols = wl.get("symbols", [])
    lines.append("# Watchlist")
    lines.append(f"WATCHLIST_PRESET={wl.get('preset', '')}")
    lines.append(f"WATCHLIST_SYMBOLS={','.join(symbols)}")
    lines.append("")

    # Notifications
    notif = config.get("notifications", {})
    lines.append("# Notifications")
    lines.append(f"NOTIFICATION_EMAIL={notif.get('email', '')}")
    lines.append(f"NOTIFICATION_SMS_PHONE={notif.get('sms_phone', '')}")
    lines.append(f"NOTIFICATION_SLACK_WEBHOOK={notif.get('slack_webhook', '')}")
    lines.append(f"NOTIFICATION_DISCORD_WEBHOOK={notif.get('discord_webhook', '')}")
    lines.append(f"NOTIFICATION_TELEGRAM_BOT_TOKEN={notif.get('telegram_bot_token', '')}")
    lines.append(f"NOTIFICATION_TELEGRAM_CHAT_ID={notif.get('telegram_chat_id', '')}")
    lines.append("")

    env_content = "\n".join(lines) + "\n"
    env_path.write_text(env_content)

    return str(env_path)
