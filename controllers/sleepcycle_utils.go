package controllers

import (
	"fmt"
	"github.com/gorhill/cronexpr"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func (r *SleepCycleReconciler) getCurrentScheduledOperation(sleepCycle corev1alpha1.SleepCycle) SleepCycleOperation {
	nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, true)
	nextScheduledShutdownTimeWindow := r.getScheduleTimeWindow(nextScheduledShutdown)
	nextScheduledWakeupTimeWindow := r.getScheduleTimeWindow(nextScheduledWakeup)

	var isWithinScheduleForShutdown, isWithinScheduleForWakeup = false, false
	isWithinScheduleForShutdown = nextScheduledShutdownTimeWindow.IsScheduleWithinWindow(time.Now())

	if nextScheduledWakeup == nil {
		if !isWithinScheduleForShutdown {
			return Watch
		}

		return Shutdown
	}

	isWithinScheduleForWakeup = nextScheduledWakeupTimeWindow.IsScheduleWithinWindow(time.Now())

	if nextScheduledShutdown.Before(*nextScheduledWakeup) && isWithinScheduleForShutdown {
		return Shutdown
	}

	if nextScheduledWakeup.Before(*nextScheduledShutdown) && isWithinScheduleForWakeup {
		return WakeUp
	}

	if isWithinScheduleForShutdown && isWithinScheduleForWakeup {
		return WakeUp
	}

	return Watch
}

func (r *SleepCycleReconciler) getNextScheduledOperation(sleepCycle corev1alpha1.SleepCycle, currentOperation *SleepCycleOperation) (SleepCycleOperation, time.Duration) {
	var requeueAfter time.Duration

	if currentOperation == nil {
		*currentOperation = r.getCurrentScheduledOperation(sleepCycle)
	}

	nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, false)
	var nextOperation SleepCycleOperation

	switch *currentOperation {
	case Watch:
		if nextScheduledWakeup == nil {
			nextOperation = Shutdown
			requeueAfter = time.Until(*nextScheduledShutdown)
		} else {
			if nextScheduledShutdown.Before(*nextScheduledWakeup) {
				nextOperation = Shutdown
				requeueAfter = time.Until(*nextScheduledShutdown)
			} else {
				nextOperation = WakeUp
				requeueAfter = time.Until(*nextScheduledWakeup)
			}
		}
	case Shutdown:
		if nextScheduledWakeup == nil {
			nextOperation = Shutdown
			requeueAfter = time.Until(*nextScheduledShutdown)
		} else {
			nextOperation = WakeUp
			requeueAfter = time.Until(*nextScheduledWakeup)
		}
	case WakeUp:
		nextOperation = Shutdown
		requeueAfter = time.Until(*nextScheduledShutdown)
	}

	return nextOperation, requeueAfter
}

func (r *SleepCycleReconciler) getScheduleTimeWindow(timestamp *time.Time) *TimeWindow {
	if timestamp != nil {
		return NewTimeWindow(*timestamp)
	}

	return nil
}

func (r *SleepCycleReconciler) getSchedulesTime(sleepCycle corev1alpha1.SleepCycle, useStatus bool) (shutdown *time.Time, wakeup *time.Time) {
	shutdown = nil
	wakeup = nil

	if !useStatus {
		shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
		wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
	} else {
		if sleepCycle.Status.NextScheduledWakeupTime != nil {
			wakeupTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledWakeupTime.Time)

			if wakeupTimeWindow.Right.Before(time.Now()) {
				wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
			} else {
				wakeup = &sleepCycle.Status.NextScheduledWakeupTime.Time
			}
		} else {
			wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
		}

		if sleepCycle.Status.NextScheduledShutdownTime != nil {
			shutdownTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledShutdownTime.Time)

			if shutdownTimeWindow.Right.Before(time.Now()) {
				shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
			} else {
				shutdown = &sleepCycle.Status.NextScheduledShutdownTime.Time
			}
		} else {
			shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
		}
	}

	return shutdown, wakeup
}

func (r *SleepCycleReconciler) getTimeFromCronExpression(cronexp string, timezone *string) *time.Time {
	tz := r.getTimeZone(timezone)

	cronExpression, err := cronexpr.Parse(cronexp)
	if err == nil {
		t := cronExpression.Next(time.Now().In(tz))
		return &t
	}
	return nil
}

func (r *SleepCycleReconciler) getTimeZone(timezone *string) *time.Location {
	tz, err := time.LoadLocation(*timezone)
	if err != nil {
		r.logger.Info(fmt.Sprintf("no valid timezone, reverting to UTC: %s", err.Error()))
		tz, _ = time.LoadLocation("UTC")
	}

	return tz
}

func (r *SleepCycleReconciler) isAnnotated(obj *metav1.ObjectMeta, tag string) bool {
	val, ok := obj.GetLabels()[SleepCycleLabel]

	if ok && val == tag {
		return true
	}

	return false
}
