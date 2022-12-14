/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	SleepCycleLabel = "rekuberate.io/sleepcycle"
)

// SleepCycleReconciler reconciles a SleepCycle object
type SleepCycleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	logger logr.Logger
}

const (
	TimeWindowToleranceInSeconds int = 30
)

//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SleepCycle object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SleepCycleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.Log.WithValues("namespace", req.Namespace, "sleepcycle", req.Name)

	var sleepCycle corev1alpha1.SleepCycle
	if err := r.Get(ctx, req.NamespacedName, &sleepCycle); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Error(err, "??????? unable to find SleepCycle")
			return ctrl.Result{}, nil
		}

		r.logger.Error(err, "???? unable to fetch SleepCycle")
		return ctrl.Result{}, err
	}

	thisRunIsSuccessful := true
	lastRunWasSuccessful := sleepCycle.Status.LastRunWasSuccessful
	isScheduledReentry := !r.isNotScheduledReentry(sleepCycle)
	currentOperation := r.getCurrentScheduledOperation(sleepCycle)
	sleepCycleFullName := fmt.Sprintf("%v/%v", sleepCycle.Namespace, sleepCycle.Name)

	if !sleepCycle.Spec.Enabled {
		return ctrl.Result{}, nil
	}

	deepCopy := *sleepCycle.DeepCopy()
	if deepCopy.Status.UsedBy == nil {
		usedBy := make(map[string]int)
		deepCopy.Status.UsedBy = usedBy
	}

	r.logger = r.logger.WithValues("op", currentOperation.String())

	if isScheduledReentry || !lastRunWasSuccessful {
		var err error

		_, err = r.ReconcileDeployments(ctx, req, &sleepCycle, &deepCopy, currentOperation)
		if err != nil {
			thisRunIsSuccessful = false
		}

		_, err = r.ReconcileCronJobs(ctx, req, &sleepCycle, currentOperation)
		if err != nil {
			thisRunIsSuccessful = false
		}

		_, err = r.ReconcileStatefulSets(ctx, req, &sleepCycle, &deepCopy, currentOperation)
		if err != nil {
			thisRunIsSuccessful = false
		}

		_, err = r.ReconcileHorizontalPodAutoscalers(ctx, req, &sleepCycle, &deepCopy, currentOperation)
		if err != nil {
			thisRunIsSuccessful = false
		}
	}

	nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, false)
	deepCopy.Status.NextScheduledShutdownTime = &metav1.Time{Time: *nextScheduledShutdown}

	if currentOperation != Watch {
		deepCopy.Status.LastRunTime = &metav1.Time{Time: time.Now()}
		deepCopy.Status.LastRunWasSuccessful = thisRunIsSuccessful
	}

	if nextScheduledWakeup != nil {
		deepCopy.Status.NextScheduledWakeupTime = &metav1.Time{Time: *nextScheduledWakeup}
	}

	if err := r.Status().Update(ctx, &deepCopy); err != nil {
		r.logger.Error(err, "??????? failed to update SleepCycle Status", "sleepcycle", sleepCycleFullName)
		return ctrl.Result{}, err
	}

	nextOperation, requeueAfter := r.getNextScheduledOperation(sleepCycle, &currentOperation)

	if currentOperation == Watch {
		r.logger.V(10).Info("???? Requeue", "next-op", nextOperation.String(), "after", requeueAfter)
	} else {
		r.logger.Info("???? Requeue", "next-op", nextOperation.String(), "after", requeueAfter)
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *SleepCycleReconciler) ScaleDeployment(ctx context.Context, deployment appsv1.Deployment, replicas int32) error {
	deepCopy := *deployment.DeepCopy()
	*deepCopy.Spec.Replicas = replicas

	if err := r.Update(ctx, &deepCopy); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) ScaleStatefulSet(ctx context.Context, statefulSet appsv1.StatefulSet, replicas int32) error {
	deepCopy := *statefulSet.DeepCopy()
	*deepCopy.Spec.Replicas = replicas

	if err := r.Update(ctx, &deepCopy); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) ScaleHorizontalPodAutoscaler(ctx context.Context, hpa autoscalingv1.HorizontalPodAutoscaler, replicas int32) error {
	deepCopy := *hpa.DeepCopy()
	deepCopy.Spec.MaxReplicas = replicas

	if err := r.Update(ctx, &deepCopy); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) SuspendCronJob(ctx context.Context, cronJob batchv1.CronJob, suspend bool) error {
	deepCopy := *cronJob.DeepCopy()
	*deepCopy.Spec.Suspend = suspend

	if err := r.Update(ctx, &deepCopy); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) WatchDeploymentsHandler(o client.Object) []ctrl.Request {
	var request []ctrl.Request

	sleepCycleList := corev1alpha1.SleepCycleList{}
	err := r.Client.List(context.Background(), &sleepCycleList)
	if err != nil {
		return nil
	}

	for _, sleepCycle := range sleepCycleList.Items {
		if !strings.HasPrefix(sleepCycle.Namespace, "kube-") {
			request = append(request, ctrl.Request{
				NamespacedName: client.ObjectKey{Namespace: sleepCycle.Namespace, Name: sleepCycle.Name},
			})
		}
	}

	return request
}

// SetupWithManager sets up the controller with the Manager.
func (r *SleepCycleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.SleepCycle{}).
		Complete(r)
}

func (r *SleepCycleReconciler) ReconcileDeployments(
	ctx context.Context,
	req ctrl.Request,
	sleepCycle *corev1alpha1.SleepCycle,
	deepCopy *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error) {
	deploymentList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deploymentList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("???? Processing Deployments")

	for _, deployment := range deploymentList.Items {
		hasSleepCycle := r.isTagged(&deployment.ObjectMeta, sleepCycle.Name)

		if hasSleepCycle {
			deploymentFullName := fmt.Sprintf("%v/%v", deployment.Namespace, deployment.Name)
			deepCopy.Status.Enabled = sleepCycle.Spec.Enabled

			currentReplicas := int(deployment.Status.Replicas)
			val, ok := deepCopy.Status.UsedBy[deploymentFullName]
			if !ok || (ok && val < currentReplicas && currentReplicas > 0) {
				deepCopy.Status.UsedBy[deploymentFullName] = currentReplicas
			}

			switch op {
			case Watch:
			case Shutdown:
				if deployment.Status.Replicas != 0 {
					r.logger.Info("???  Scale Down Deployment", "deployment", deploymentFullName, "targetReplicas", 0)

					err := r.ScaleDeployment(ctx, deployment, 0)
					if err != nil {
						r.logger.Error(err, "??????? Scaling Deployment failed", "deployment", deploymentFullName)
						return ctrl.Result{}, err
					}
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[deploymentFullName])

				if deployment.Status.Replicas != targetReplicas {
					r.logger.Info("???  Scale Up Deployment", "deployment", deploymentFullName, "targetReplicas", targetReplicas)

					err := r.ScaleDeployment(ctx, deployment, targetReplicas)
					if err != nil {
						r.logger.Error(err, "??????? Scaling Deployment failed", "deployment", deploymentFullName)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) ReconcileCronJobs(ctx context.Context,
	req ctrl.Request,
	sleepCycle *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error) {
	cronJobList := batchv1.CronJobList{}
	if err := r.List(ctx, &cronJobList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("???? Processing CronJobs")

	for _, cronJob := range cronJobList.Items {
		hasSleepCycle := r.isTagged(&cronJob.ObjectMeta, sleepCycle.Name)

		if hasSleepCycle {
			cronJobFullName := fmt.Sprintf("%v/%v", cronJob.Namespace, cronJob.Name)

			switch op {
			case Watch:
			case Shutdown:
				if !*cronJob.Spec.Suspend {
					r.logger.Info("???  Suspending CronJob", "cronJob", cronJobFullName)

					err := r.SuspendCronJob(ctx, cronJob, true)
					if err != nil {
						r.logger.Error(err, "?????????? Suspending CronJob failed", "cronJob", cronJobFullName)
						return ctrl.Result{}, err
					}
				}
			case WakeUp:
				if *cronJob.Spec.Suspend {
					r.logger.Info("???  Enabling Cronjob", "cronJob", cronJobFullName)

					err := r.SuspendCronJob(ctx, cronJob, false)
					if err != nil {
						r.logger.Error(err, "?????????? Suspending CronJob failed", "cronJob", cronJobFullName)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) ReconcileStatefulSets(
	ctx context.Context,
	req ctrl.Request,
	sleepCycle *corev1alpha1.SleepCycle,
	deepCopy *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error) {
	statefulSetList := appsv1.StatefulSetList{}
	if err := r.List(ctx, &statefulSetList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("???? Processing StatefulSets")

	for _, statefulSet := range statefulSetList.Items {
		hasSleepCycle := r.isTagged(&statefulSet.ObjectMeta, sleepCycle.Name)

		if hasSleepCycle {
			statefulSetFullName := fmt.Sprintf("%v/%v", statefulSet.Namespace, statefulSet.Name)
			deepCopy.Status.Enabled = sleepCycle.Spec.Enabled

			currentReplicas := int(statefulSet.Status.Replicas)
			val, ok := deepCopy.Status.UsedBy[statefulSetFullName]
			if !ok || (ok && val < currentReplicas && currentReplicas > 0) {
				deepCopy.Status.UsedBy[statefulSetFullName] = currentReplicas
			}

			switch op {
			case Watch:
			case Shutdown:
				if statefulSet.Status.Replicas != 0 {
					r.logger.Info("???  Scale Down StatefulSet", "statefulSet", statefulSetFullName, "targetReplicas", 0)

					err := r.ScaleStatefulSet(ctx, statefulSet, 0)
					if err != nil {
						r.logger.Error(err, "??????? Scaling StatefulSet failed", "statefulSet", statefulSetFullName)
						return ctrl.Result{}, err
					}
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[statefulSetFullName])

				if statefulSet.Status.Replicas != targetReplicas {
					r.logger.Info("???  Scale Up StatefulSet", "statefulSet", statefulSetFullName, "targetReplicas", targetReplicas)

					err := r.ScaleStatefulSet(ctx, statefulSet, targetReplicas)
					if err != nil {
						r.logger.Error(err, "??????? Scaling StatefulSet failed", "statefulSet", statefulSetFullName)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) ReconcileHorizontalPodAutoscalers(
	ctx context.Context,
	req ctrl.Request,
	sleepCycle *corev1alpha1.SleepCycle,
	deepCopy *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error) {
	hpaList := autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.List(ctx, &hpaList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("???? Processing HorizontalPodAutoscalers")

	for _, hpa := range hpaList.Items {
		hasSleepCycle := r.isTagged(&hpa.ObjectMeta, sleepCycle.Name)

		if hasSleepCycle {
			hpaFullName := fmt.Sprintf("%v/%v", hpa.Namespace, hpa.Name)
			deepCopy.Status.Enabled = sleepCycle.Spec.Enabled

			maxReplicas := int(hpa.Spec.MaxReplicas)
			val, ok := deepCopy.Status.UsedBy[hpaFullName]
			if !ok || (ok && val < maxReplicas && maxReplicas > 0) {
				deepCopy.Status.UsedBy[hpaFullName] = maxReplicas
			}

			switch op {
			case Watch:
			case Shutdown:
				if hpa.Spec.MaxReplicas != 1 {
					r.logger.Info("???  Scale Down HorizontalPodAutoscaler", "hpa", hpaFullName, "maxReplicas", 1)

					err := r.ScaleHorizontalPodAutoscaler(ctx, hpa, 1)
					if err != nil {
						r.logger.Error(err, "??????? Scaling HorizontalPodAutoscaler failed", "hpa", hpaFullName)
						return ctrl.Result{}, err
					}
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[hpaFullName])

				if hpa.Spec.MaxReplicas != targetReplicas {
					r.logger.Info("???  Scale Up HorizontalPodAutoscaler", "hpa", hpaFullName, "maxReplicas", targetReplicas)

					err := r.ScaleHorizontalPodAutoscaler(ctx, hpa, targetReplicas)
					if err != nil {
						r.logger.Error(err, "??????? Scaling HorizontalPodAutoscaler failed", "hpa", hpaFullName)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) getCurrentScheduledOperation(sleepCycle corev1alpha1.SleepCycle) SleepCycleOperation {
	nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(sleepCycle, true)
	nextScheduledShutdownTimeWindow := r.getScheduleTimeWindow(nextScheduledShutdown)
	nextScheduledWakeupTimeWindow := r.getScheduleTimeWindow(nextScheduledWakeup)

	var isWithinScheduleForShutdown, isWithinScheduleForWakeup = false, false
	isWithinScheduleForShutdown = nextScheduledShutdownTimeWindow.IsScheduleWithinWindow(time.Now())

	if nextScheduledWakeup == nil && isWithinScheduleForShutdown {
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
		shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown)
		wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp)
	} else {
		if sleepCycle.Status.NextScheduledWakeupTime != nil {
			wakeupTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledWakeupTime.Time)

			if wakeupTimeWindow.Right.Before(time.Now()) {
				wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp)
			} else {
				wakeup = &sleepCycle.Status.NextScheduledWakeupTime.Time
			}
		} else {
			wakeup = r.getTimeFromCronExpression(sleepCycle.Spec.WakeUp)
		}

		if sleepCycle.Status.NextScheduledShutdownTime != nil {
			shutdownTimeWindow := NewTimeWindow(sleepCycle.Status.NextScheduledShutdownTime.Time)

			if shutdownTimeWindow.Right.Before(time.Now()) {
				shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown)
			} else {
				shutdown = &sleepCycle.Status.NextScheduledShutdownTime.Time
			}
		} else {
			shutdown = r.getTimeFromCronExpression(sleepCycle.Spec.Shutdown)
		}
	}

	return shutdown, wakeup
}

func (r *SleepCycleReconciler) getTimeFromCronExpression(cronexp string) *time.Time {
	cronExpression, err := cronexpr.Parse(cronexp)
	if err == nil {
		t := cronExpression.Next(time.Now())
		return &t
	}
	return nil
}

func (r *SleepCycleReconciler) isNotScheduledReentry(sleepCycle corev1alpha1.SleepCycle) bool {
	now := metav1.Time{Time: time.Now()}

	if sleepCycle.Status.NextScheduledShutdownTime == nil {
		return false
	}

	if now.Time.Before(sleepCycle.Status.NextScheduledShutdownTime.Time) && sleepCycle.Status.NextScheduledWakeupTime == nil {
		return true
	}

	if now.Time.Before(sleepCycle.Status.NextScheduledShutdownTime.Time) && (sleepCycle.Status.NextScheduledWakeupTime != nil && now.Before(sleepCycle.Status.NextScheduledWakeupTime)) {
		return true
	}

	return false
}

func (r *SleepCycleReconciler) isTagged(obj *metav1.ObjectMeta, tag string) bool {
	val, ok := obj.GetLabels()[SleepCycleLabel]

	if ok && val == tag {
		return true
	}

	return false
}
