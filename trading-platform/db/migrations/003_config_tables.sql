-- Seed default configuration values
INSERT INTO app_config (key, value, category, label, description, value_type, constraints) VALUES
-- Broker settings
('broker.provider', '"alpaca"', 'broker', 'Broker', 'Which broker to use for trading', 'select',
 '{"options": ["alpaca", "ibkr", "tradier", "schwab"]}'),
('broker.api_key', '""', 'broker', 'API Key', 'Your broker API key', 'string', NULL),
('broker.api_secret', '""', 'broker', 'API Secret', 'Your broker API secret (stored encrypted)', 'string', NULL),
('broker.paper_mode', 'true', 'broker', 'Paper Trading Mode', 'When enabled, trades use simulated money — no real orders are placed', 'boolean', NULL),

-- Risk settings
('risk.max_daily_loss', '500', 'risk', 'Maximum Daily Loss ($)', 'If your losses reach this amount in a single day, all trading stops automatically', 'number',
 '{"min": 50, "max": 100000, "step": 50}'),
('risk.max_position_pct', '5', 'risk', 'Max Position Size (%)', 'Maximum percentage of your portfolio in any single trade', 'number',
 '{"min": 1, "max": 25, "step": 1}'),
('risk.max_portfolio_risk_pct', '15', 'risk', 'Max Total Portfolio Risk (%)', 'Maximum total risk across all open positions', 'number',
 '{"min": 5, "max": 50, "step": 5}'),
('risk.require_stop_loss', 'true', 'risk', 'Require Stop Loss', 'Every trade must have a stop-loss order — protects against large single-trade losses', 'boolean', NULL),
('risk.max_orders_per_minute', '10', 'risk', 'Max Orders Per Minute', 'Rate limit to prevent runaway algorithms', 'number',
 '{"min": 1, "max": 60, "step": 1}'),

-- Data provider settings
('data.provider', '"alpaca"', 'data', 'Market Data Provider', 'Where real-time price data comes from', 'select',
 '{"options": ["alpaca", "polygon", "tradier", "alpha_vantage"]}'),
('data.api_key', '""', 'data', 'Data API Key', 'API key for your market data provider', 'string', NULL),

-- Notification settings
('notifications.email', '""', 'notifications', 'Email Address', 'Where to send email alerts', 'string', NULL),
('notifications.sms_phone', '""', 'notifications', 'Phone Number', 'SMS alerts for critical events (requires Twilio)', 'string', NULL),
('notifications.slack_webhook', '""', 'notifications', 'Slack Webhook URL', 'Post alerts to a Slack channel', 'string', NULL),
('notifications.discord_webhook', '""', 'notifications', 'Discord Webhook URL', 'Post alerts to a Discord channel', 'string', NULL),
('notifications.telegram_bot_token', '""', 'notifications', 'Telegram Bot Token', 'Telegram bot for alerts', 'string', NULL),
('notifications.telegram_chat_id', '""', 'notifications', 'Telegram Chat ID', 'Telegram chat to post alerts to', 'string', NULL),

-- Watchlist
('watchlist.symbols', '["SPY","QQQ","AAPL","MSFT","NVDA","AMZN","GOOG","TSLA","META","JPM"]', 'watchlist',
 'Watchlist', 'Symbols you want to monitor and trade', 'json', NULL),

-- System settings
('system.log_level', '"info"', 'system', 'Log Level', 'How verbose the system logs are', 'select',
 '{"options": ["debug", "info", "warning", "error"]}'),
('system.auto_update', 'true', 'system', 'Auto-Update', 'Check for updates automatically and notify when available', 'boolean', NULL),
('system.backup_reminder_days', '30', 'system', 'Backup Reminder (days)', 'How often to remind you to back up your data', 'number',
 '{"min": 7, "max": 90, "step": 7}')

ON CONFLICT (key) DO NOTHING;
