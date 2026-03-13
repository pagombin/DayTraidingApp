package calendar

import (
	"fmt"
	"time"
)

type USMarketCalendar struct {
	holidays    map[string]string
	earlyCloses map[string]bool
	loc         *time.Location
}

func NewUSMarketCalendar() *USMarketCalendar {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback: use fixed offset EST (UTC-5)
		loc = time.FixedZone("EST", -5*3600)
	}
	return &USMarketCalendar{
		holidays:    usHolidays,
		earlyCloses: usEarlyCloses,
		loc:         loc,
	}
}

// IsMarketOpen returns true if the market is in regular trading hours.
func (c *USMarketCalendar) IsMarketOpen(t time.Time) bool {
	et := t.In(c.loc)
	if !c.IsTradingDay(et) {
		return false
	}

	minutes := et.Hour()*60 + et.Minute()
	open := 9*60 + 30   // 9:30 AM
	close := 16 * 60     // 4:00 PM

	if c.isEarlyClose(et) {
		close = 13 * 60 // 1:00 PM
	}

	return minutes >= open && minutes < close
}

// IsPreMarket returns true if currently in pre-market hours (4:00 AM - 9:30 AM ET).
func (c *USMarketCalendar) IsPreMarket(t time.Time) bool {
	et := t.In(c.loc)
	if !c.IsTradingDay(et) {
		return false
	}
	minutes := et.Hour()*60 + et.Minute()
	return minutes >= 4*60 && minutes < 9*60+30
}

// IsAfterHours returns true if currently in after-hours (4:00 PM - 8:00 PM ET).
func (c *USMarketCalendar) IsAfterHours(t time.Time) bool {
	et := t.In(c.loc)
	if !c.IsTradingDay(et) {
		return false
	}
	minutes := et.Hour()*60 + et.Minute()
	close := 16 * 60
	if c.isEarlyClose(et) {
		close = 13 * 60
	}
	return minutes >= close && minutes < 20*60
}

// IsTradingDay returns true if the given date is a trading day.
func (c *USMarketCalendar) IsTradingDay(d time.Time) bool {
	et := d.In(c.loc)
	day := et.Weekday()
	if day == time.Saturday || day == time.Sunday {
		return false
	}
	dateStr := et.Format("2006-01-02")
	_, isHoliday := c.holidays[dateStr]
	return !isHoliday
}

// NextOpen returns the next market open time.
func (c *USMarketCalendar) NextOpen(t time.Time) time.Time {
	et := t.In(c.loc)
	// Check if market is currently before open today
	if c.IsTradingDay(et) {
		todayOpen := time.Date(et.Year(), et.Month(), et.Day(), 9, 30, 0, 0, c.loc)
		if et.Before(todayOpen) {
			return todayOpen
		}
	}
	// Find next trading day
	next := time.Date(et.Year(), et.Month(), et.Day()+1, 9, 30, 0, 0, c.loc)
	for i := 0; i < 10; i++ { // max 10 days lookahead
		if c.IsTradingDay(next) {
			return next
		}
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// NextClose returns the next market close time.
func (c *USMarketCalendar) NextClose(t time.Time) time.Time {
	et := t.In(c.loc)
	closeHour := 16
	if c.isEarlyClose(et) {
		closeHour = 13
	}
	todayClose := time.Date(et.Year(), et.Month(), et.Day(), closeHour, 0, 0, 0, c.loc)
	if c.IsTradingDay(et) && et.Before(todayClose) {
		return todayClose
	}
	// Find next trading day's close
	next := time.Date(et.Year(), et.Month(), et.Day()+1, 0, 0, 0, 0, c.loc)
	for i := 0; i < 10; i++ {
		if c.IsTradingDay(next) {
			ch := 16
			if c.isEarlyClose(next) {
				ch = 13
			}
			return time.Date(next.Year(), next.Month(), next.Day(), ch, 0, 0, 0, c.loc)
		}
		next = next.AddDate(0, 0, 1)
	}
	return todayClose
}

// MinutesUntilClose returns minutes until market close. Returns 0 if market is closed.
func (c *USMarketCalendar) MinutesUntilClose(t time.Time) int {
	if !c.IsMarketOpen(t) {
		return 0
	}
	et := t.In(c.loc)
	closeHour := 16
	if c.isEarlyClose(et) {
		closeHour = 13
	}
	closeTime := time.Date(et.Year(), et.Month(), et.Day(), closeHour, 0, 0, 0, c.loc)
	diff := closeTime.Sub(et)
	return int(diff.Minutes())
}

// IsMonthlyOpex returns true if the given date is the 3rd Friday of the month.
func (c *USMarketCalendar) IsMonthlyOpex(d time.Time) bool {
	et := d.In(c.loc)
	if et.Weekday() != time.Friday {
		return false
	}
	// 3rd Friday: day must be between 15-21
	return et.Day() >= 15 && et.Day() <= 21
}

// MarketStatus returns a human-readable status string.
func (c *USMarketCalendar) MarketStatus(t time.Time) string {
	et := t.In(c.loc)
	dateStr := et.Format("2006-01-02")

	if name, ok := c.holidays[dateStr]; ok {
		return fmt.Sprintf("Holiday: %s", name)
	}

	if c.IsMarketOpen(t) {
		mins := c.MinutesUntilClose(t)
		hours := mins / 60
		remaining := mins % 60
		if hours > 0 {
			return fmt.Sprintf("Open — closes in %dh %dm", hours, remaining)
		}
		return fmt.Sprintf("Open — closes in %dm", remaining)
	}

	if c.IsPreMarket(t) {
		return "Pre-Market"
	}

	if c.IsAfterHours(t) {
		return "After-Hours"
	}

	nextOpen := c.NextOpen(t)
	if nextOpen.Day() == et.Day() {
		return fmt.Sprintf("Closed — opens today at %s", nextOpen.Format("3:04 PM"))
	}
	return fmt.Sprintf("Closed — opens %s %s", nextOpen.Format("Mon"), nextOpen.Format("3:04 PM"))
}

// HolidayName returns the holiday name if the date is a holiday, or empty string.
func (c *USMarketCalendar) HolidayName(t time.Time) string {
	et := t.In(c.loc)
	return c.holidays[et.Format("2006-01-02")]
}

func (c *USMarketCalendar) isEarlyClose(et time.Time) bool {
	return c.earlyCloses[et.Format("2006-01-02")]
}
