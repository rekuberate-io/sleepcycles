package controllers

import (
	"time"
)

func NewTimeWindow(schedule time.Time) (timeWindow *TimeWindow) {
	left := schedule.Add(-(time.Duration(TimeWindowToleranceInSeconds) * time.Second))
	right := schedule.Add(time.Duration(TimeWindowToleranceInSeconds) * time.Second)

	return &TimeWindow{
		Left:  left,
		Right: right,
	}
}

type TimeWindow struct {
	Left  time.Time
	Right time.Time
}

func (tw TimeWindow) IsScheduleWithinWindow(schedule time.Time) bool {
	if schedule.After(tw.Left) && schedule.Before(tw.Right) {
		return true
	}

	return false
}
