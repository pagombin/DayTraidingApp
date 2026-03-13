package calendar

// US market holidays for 2025-2027 (NYSE/Nasdaq).
// Source: NYSE holiday calendar.
var usHolidays = map[string]string{
	// 2025
	"2025-01-01": "New Year's Day",
	"2025-01-20": "MLK Jr. Day",
	"2025-02-17": "Presidents' Day",
	"2025-04-18": "Good Friday",
	"2025-05-26": "Memorial Day",
	"2025-06-19": "Juneteenth",
	"2025-07-04": "Independence Day",
	"2025-09-01": "Labor Day",
	"2025-11-27": "Thanksgiving Day",
	"2025-12-25": "Christmas Day",
	// 2026
	"2026-01-01": "New Year's Day",
	"2026-01-19": "MLK Jr. Day",
	"2026-02-16": "Presidents' Day",
	"2026-04-03": "Good Friday",
	"2026-05-25": "Memorial Day",
	"2026-06-19": "Juneteenth",
	"2026-07-03": "Independence Day (observed)",
	"2026-09-07": "Labor Day",
	"2026-11-26": "Thanksgiving Day",
	"2026-12-25": "Christmas Day",
	// 2027
	"2027-01-01": "New Year's Day",
	"2027-01-18": "MLK Jr. Day",
	"2027-02-15": "Presidents' Day",
	"2027-03-26": "Good Friday",
	"2027-05-31": "Memorial Day",
	"2027-06-18": "Juneteenth (observed)",
	"2027-07-05": "Independence Day (observed)",
	"2027-09-06": "Labor Day",
	"2027-11-25": "Thanksgiving Day",
	"2027-12-24": "Christmas Day (observed)",
}

// Early closes (1:00 PM ET)
var usEarlyCloses = map[string]bool{
	// 2025
	"2025-07-03": true, // Day before Independence Day
	"2025-11-28": true, // Black Friday
	"2025-12-24": true, // Christmas Eve
	// 2026
	"2026-07-02": true,
	"2026-11-27": true,
	"2026-12-24": true,
	// 2027
	"2027-07-02": true,
	"2027-11-26": true,
	"2027-12-23": true,
}
