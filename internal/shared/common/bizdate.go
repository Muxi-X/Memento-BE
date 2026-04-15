package common

import "time"

const BusinessTimezone = "Asia/Shanghai"

var businessLocation = loadBusinessLocation()

func BusinessLocation() *time.Location {
	return businessLocation
}

func NormalizeBizDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	local := t.In(businessLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func loadBusinessLocation() *time.Location {
	loc, err := time.LoadLocation(BusinessTimezone)
	if err == nil {
		return loc
	}
	return time.FixedZone(BusinessTimezone, 8*60*60)
}
