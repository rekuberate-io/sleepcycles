package controllers

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func (r *SleepCycleReconciler) hasLabel(obj *metav1.ObjectMeta, tag string) bool {
	val, ok := obj.GetLabels()[SleepCycleLabel]

	if ok && val == tag {
		return true
	}

	return false
}

//	func (r *SleepCycleReconciler) getCurrentScheduledOperation(sleepCycle corev1alpha1.SleepCycle) SleepCycleOperation {
//		nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, true)
//		nextScheduledShutdownTimeWindow := r.getScheduleTimeWindow(nextScheduledShutdown)
//		nextScheduledWakeupTimeWindow := r.getScheduleTimeWindow(nextScheduledWakeup)
//
//		var isWithinScheduleForShutdown, isWithinScheduleForWakeup = false, false
//		isWithinScheduleForShutdown = nextScheduledShutdownTimeWindow.IsScheduleWithinWindow(time.Now())
//
//		if nextScheduledWakeup == nil {
//			if !isWithinScheduleForShutdown {
//				return Watch
//			}
//
//			return Shutdown
//		}
//
//		isWithinScheduleForWakeup = nextScheduledWakeupTimeWindow.IsScheduleWithinWindow(time.Now())
//
//		if nextScheduledShutdown.Before(*nextScheduledWakeup) && isWithinScheduleForShutdown {
//			return Shutdown
//		}
//
//		if nextScheduledWakeup.Before(*nextScheduledShutdown) && isWithinScheduleForWakeup {
//			return WakeUp
//		}
//
//		if isWithinScheduleForShutdown && isWithinScheduleForWakeup {
//			return WakeUp
//		}
//
//		return Watch
//	}
//
//	func (r *SleepCycleReconciler) getNextScheduledOperation(sleepCycle corev1alpha1.SleepCycle, currentOperation *SleepCycleOperation) (SleepCycleOperation, time.Duration) {
//		var requeueAfter time.Duration
//
//		if currentOperation == nil {
//			*currentOperation = r.getCurrentScheduledOperation(sleepCycle)
//		}
//
//		nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, false)
//		var nextOperation SleepCycleOperation
//
//		switch *currentOperation {
//		case Watch:
//			if nextScheduledWakeup == nil {
//				nextOperation = Shutdown
//				requeueAfter = time.Until(*nextScheduledShutdown)
//			} else {
//				if nextScheduledShutdown.Before(*nextScheduledWakeup) {
//					nextOperation = Shutdown
//					requeueAfter = time.Until(*nextScheduledShutdown)
//				} else {
//					nextOperation = WakeUp
//					requeueAfter = time.Until(*nextScheduledWakeup)
//				}
//			}
//		case Shutdown:
//			if nextScheduledWakeup == nil {
//				nextOperation = Shutdown
//				requeueAfter = time.Until(*nextScheduledShutdown)
//			} else {
//				nextOperation = WakeUp
//				requeueAfter = time.Until(*nextScheduledWakeup)
//			}
//		case WakeUp:
//			nextOperation = Shutdown
//			requeueAfter = time.Until(*nextScheduledShutdown)
//		}
//
//		return nextOperation, requeueAfter
//	}
//
//	func (r *SleepCycleReconciler) getScheduleTimeWindow(timestamp *time.Time) *TimeWindow {
//		if timestamp != nil {
//			return NewTimeWindow(*timestamp)
//		}
//
//		return nil
//	}
//
//	func (r *SleepCycleReconciler) getSchedulesTime(sleepCycle corev1alpha1.SleepCycle, useStatus bool) (shutdown *time.Time, wakeup *time.Time) {
//		shutdown = nil
//		wakeup = nil
//
//		if !useStatus {
//			shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
//			wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
//		} else {
//			if sleepCycle.Status.NextScheduledWakeupTime != nil {
//				wakeupTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledWakeupTime.Time)
//
//				if wakeupTimeWindow.Right.Before(time.Now()) {
//					wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
//				} else {
//					wakeup = &sleepCycle.Status.NextScheduledWakeupTime.Time
//				}
//			} else {
//				wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp, sleepCycle.Spec.WakeupTimeZone)
//			}
//
//			if sleepCycle.Status.NextScheduledShutdownTime != nil {
//				shutdownTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledShutdownTime.Time)
//
//				if shutdownTimeWindow.Right.Before(time.Now()) {
//					shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
//				} else {
//					shutdown = &sleepCycle.Status.NextScheduledShutdownTime.Time
//				}
//			} else {
//				shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown, sleepCycle.Spec.ShutdownTimeZone)
//			}
//		}
//
//		return shutdown, wakeup
//	}
//
//	func (r *SleepCycleReconciler) getTimeFromCronExpression(cronexp string, timezone *string) *time.Time {
//		tz := r.getTimeZone(timezone)
//
//		cronExpression, err := cronexpr.Parse(cronexp)
//		if err == nil {
//			t := cronExpression.Next(time.Now().In(tz))
//			return &t
//		}
//		return nil
//	}
//
//	func (r *SleepCycleReconciler) getTimeZone(timezone *string) *time.Location {
//		tz, err := time.LoadLocation(*timezone)
//		if err != nil {
//			r.logger.Info(fmt.Sprintf("no valid timezone, reverting to UTC: %s", err.Error()))
//			tz, _ = time.LoadLocation("UTC")
//		}
//
//		return tz
//	}

//
//func (r *SleepCycleReconciler) removeLabel(obj *metav1.ObjectMeta, tag string) bool {
//	val, ok := obj.GetLabels()[SleepCycleLabel]
//
//	if ok && val == tag {
//		return true
//	}
//
//	return false
//}
//
//func (r *SleepCycleReconciler) refreshLabelsHorizontalPodAutoscalers(original *corev1alpha1.SleepCycle, desired *corev1alpha1.SleepCycle, hpas autoscalingv1.HorizontalPodAutoscalerList) {
//	usedBy := original.Status.UsedBy
//	if usedBy != nil {
//		for od, _ := range usedBy {
//			contains := false
//			if strings.HasPrefix(od, "(HorizontalPodAutoscaler)") {
//				for _, ed := range hpas.Items {
//					if od == fmt.Sprintf(UsedByLabelKey, ed.Kind, ed.Namespace, ed.Name) {
//						contains = true
//						break
//					}
//				}
//
//				if !contains {
//					delete(desired.Status.UsedBy, od)
//				}
//			}
//		}
//	}
//}
//
//func (r *SleepCycleReconciler) refreshLabelsStatefulSets(original *corev1alpha1.SleepCycle, desired *corev1alpha1.SleepCycle, statefulSets appsv1.StatefulSetList) {
//	usedBy := original.Status.UsedBy
//	if usedBy != nil {
//		for od, _ := range usedBy {
//			contains := false
//			if strings.HasPrefix(od, "(StatefulSet)") {
//				for _, ed := range statefulSets.Items {
//					if od == fmt.Sprintf(UsedByLabelKey, ed.Kind, ed.Namespace, ed.Name) {
//						contains = true
//						break
//					}
//				}
//
//				if !contains {
//					delete(desired.Status.UsedBy, od)
//				}
//			}
//		}
//	}
//}
//
//func (r *SleepCycleReconciler) refreshLabelsDeployments(original *corev1alpha1.SleepCycle, desired *corev1alpha1.SleepCycle, deployments appsv1.DeploymentList) {
//	usedBy := original.Status.UsedBy
//	if usedBy != nil {
//		for od, _ := range usedBy {
//			contains := false
//			if strings.HasPrefix(od, "(Deployment)") {
//				for _, ed := range deployments.Items {
//					if od == fmt.Sprintf(UsedByLabelKey, ed.Kind, ed.Namespace, ed.Name) {
//						contains = true
//						break
//					}
//				}
//
//				if !contains {
//					delete(desired.Status.UsedBy, od)
//				}
//			}
//		}
//	}
//}
//
//func containsString(slice []string, s string) bool {
//	for _, item := range slice {
//		if item == s {
//			return true
//		}
//	}
//	return false
//}
//
//func removeString(slice []string, s string) (result []string) {
//	for _, item := range slice {
//		if item == s {
//			continue
//		}
//		result = append(result, item)
//	}
//	return
//}
