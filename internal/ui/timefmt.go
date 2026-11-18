package appui

import (
	"math"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/momaek/lastab/locales"
)

// relTime writes when t was, relative to now, as lists show it: "4 min
// ago" within the hour, hours ago within today, then the date and time.
func relTime(now, t time.Time, locale string) string {
	d := now.Sub(t)
	opt := i18n.RelativeTimeOptions{Auto: true, Style: i18n.RelativeShort}
	switch {
	case d < time.Minute:
		return i18n.FormatRelativeTime(locale, 0, i18n.RelativeSecond, opt)
	case d < time.Hour:
		return i18n.FormatRelativeTime(locale, -math.Floor(d.Minutes()), i18n.RelativeMinute, opt)
	case sameDay(now, t):
		return i18n.FormatRelativeTime(locale, -math.Floor(d.Hours()), i18n.RelativeHour, opt)
	}
	return exactTime(now, t, locale)
}

// exactTime writes the date and time, without the year when it is this year.
func exactTime(now, t time.Time, locale string) string {
	skel := "MMMdjm"
	if t.Year() != now.Year() {
		skel = "yMMMdjm"
	}
	return i18n.FormatDate(locale, t.Local(), i18n.DateOptions{Skeleton: skel})
}

// dayLabel names the day of t, for grouping lists: Today, Yesterday, Nov 12.
func dayLabel(now, t time.Time, locale string) string {
	switch {
	case sameDay(now, t):
		return locales.Today()
	case sameDay(now.AddDate(0, 0, -1), t):
		return locales.Yesterday()
	case t.Year() == now.Year():
		return i18n.FormatDate(locale, t.Local(), i18n.DateOptions{Skeleton: "MMMd"})
	}
	return i18n.FormatDate(locale, t.Local(), i18n.DateOptions{Skeleton: "yMMMd"})
}

func sameDay(a, b time.Time) bool {
	a, b = a.Local(), b.Local()
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
