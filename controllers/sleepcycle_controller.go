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
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DeploymentSleepCycleLabel = "rekuberate.io/sleepcycle"
)

// SleepCycleReconciler reconciles a SleepCycle object
type SleepCycleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
	// log := log.FromContext(ctx)
	log := log.Log.WithValues("namespace", req.Namespace, "sleepcycle", req.Name)

	var sleepCycle corev1alpha1.SleepCycle
	if err := r.Get(ctx, req.NamespacedName, &sleepCycle); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "⛔️ unable to find SleepCycle")
			return ctrl.Result{}, nil
		}

		log.Error(err, "⛔️ unable to fetch SleepCycle")
		return ctrl.Result{}, err
	}

	if !sleepCycle.Spec.Enabled {
		return ctrl.Result{}, nil
	}

	sleepCycleFullName := fmt.Sprintf("%v/%v", sleepCycle.Namespace, sleepCycle.Name)

	deploymentList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deploymentList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	var updateSleepCycleStatus = false
	newSleepCycle := *sleepCycle.DeepCopy()
	currentOperation := r.GetCurrentScheduledOperation(sleepCycle)

	log = log.WithValues("op", currentOperation.String())

	for _, deployment := range deploymentList.Items {
		sleepCycleRef, hasSleepCycle := deployment.Labels[DeploymentSleepCycleLabel]

		if hasSleepCycle && sleepCycleRef == sleepCycle.Name {
			updateSleepCycleStatus = true
			deploymentFullName := fmt.Sprintf("%v/%v", deployment.Namespace, deployment.Name)

			log.Info("🔅 Processing Deployment", "deployment", deploymentFullName)

			newSleepCycle.Status.Enabled = sleepCycle.Spec.Enabled

			if newSleepCycle.Status.UsedBy == nil {
				usedBy := make(map[string]int)
				newSleepCycle.Status.UsedBy = usedBy
			}

			replicas := int(deployment.Status.Replicas)
			if replicas > 0 {
				newSleepCycle.Status.UsedBy[deploymentFullName] = replicas
			}

			switch currentOperation {
			case Watch:
			case Shutdown:
				log.Info("⬇  Scale Down Deployment", "deployment", deploymentFullName, "replicas", 0)

				err := r.ScaleDeployment(ctx, req, deployment, 0)
				if err != nil {
					log.Error(err, "⛔️ failed to shutdown Deployment", "deployment", deploymentFullName)
				}
			case WakeUp:
				wakeUpReplicas := int32(newSleepCycle.Status.UsedBy[deploymentFullName])
				log.Info("⬆  Scale Up Deployment", "deployment", deploymentFullName, "replicas", wakeUpReplicas)

				err := r.ScaleDeployment(ctx, req, deployment, wakeUpReplicas)
				if err != nil {
					log.Error(err, "✖️ failed to wakeup deployment", "deployment", deploymentFullName)
				}
			}
		}
	}

	if updateSleepCycleStatus {
		nextScheduledShutdown, nextScheduledWakeup := r.GetSchedulesTime(sleepCycle, false)
		newSleepCycle.Status.NextScheduledShutdownTime = &v1.Time{Time: *nextScheduledShutdown}
		newSleepCycle.Status.LastReconciliationLoop = &v1.Time{Time: time.Now()}

		if nextScheduledWakeup != nil {
			newSleepCycle.Status.NextScheduledWakeupTime = &v1.Time{Time: *nextScheduledWakeup}
		}

		if err := r.Status().Update(ctx, &newSleepCycle); err != nil {
			log.Error(err, "✖️ failed to update SleepCycle Status", "sleepcycle", sleepCycleFullName)
			return ctrl.Result{}, err
		}
	}

	nextOperation, requeueAfter := r.GetNextScheduledOperation(sleepCycle)
	log.Info("🔙 Requeue", "next-op", nextOperation.String(), "after", requeueAfter)

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *SleepCycleReconciler) ScaleDeployment(ctx context.Context, req ctrl.Request, deployment appsv1.Deployment, replicas int32) error {
	deepCopy := *deployment.DeepCopy()
	*deepCopy.Spec.Replicas = replicas

	if err := r.Update(ctx, &deepCopy); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) WatchDeploymentsHandler(o client.Object) []ctrl.Request {
	request := []ctrl.Request{}

	sleepCycleList := corev1alpha1.SleepCycleList{}
	r.Client.List(context.Background(), &sleepCycleList)

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
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(r.WatchDeploymentsHandler),
		).
		Complete(r)
}

func (r *SleepCycleReconciler) GetCurrentScheduledOperation(sleepCycle corev1alpha1.SleepCycle) (nextScheduledOperation SleepCycleOperation) {

	nextScheduledOperation = Watch
	nextScheduledShutdown, nextScheduledWakeup := r.GetSchedulesTime(sleepCycle, true)
	shutdownTimeWindow, wakeupTimeWindow := r.GetScheduleTimeWindows(sleepCycle, true)

	var isWithinScheduleForShutdown, isWithinScheduleForWakeup bool = false, false

	isWithinScheduleForShutdown = shutdownTimeWindow.IsScheduleWithinWindow(time.Now())

	if wakeupTimeWindow != nil {
		isWithinScheduleForWakeup = wakeupTimeWindow.IsScheduleWithinWindow(time.Now())
	}

	if nextScheduledWakeup == nil {
		nextScheduledOperation = Shutdown
		return nextScheduledOperation
	}

	if nextScheduledShutdown.Before(*nextScheduledWakeup) && isWithinScheduleForShutdown {
		nextScheduledOperation = Shutdown
		return nextScheduledOperation
	}

	if nextScheduledWakeup.Before(*nextScheduledShutdown) && isWithinScheduleForWakeup {
		nextScheduledOperation = WakeUp
		return nextScheduledOperation
	}

	if isWithinScheduleForShutdown && isWithinScheduleForWakeup {
		nextScheduledOperation = WakeUp
	}

	return nextScheduledOperation
}

func (r *SleepCycleReconciler) GetNextScheduledOperation(sleepCycle corev1alpha1.SleepCycle) (SleepCycleOperation, time.Duration) {
	var requeueAfter time.Duration
	currentOperation := r.GetCurrentScheduledOperation(sleepCycle)
	nextScheduledShutdown, nextScheduledWakeup := r.GetSchedulesTime(sleepCycle, false)
	var nextOperation SleepCycleOperation

	switch currentOperation {
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

func (r *SleepCycleReconciler) GetScheduleTimeWindows(sleepCycle corev1alpha1.SleepCycle, useStatus bool) (shutdown *TimeWindow, wakeup *TimeWindow) {
	nextScheduledShutdown, nextScheduledWakeup := r.GetSchedulesTime(sleepCycle, useStatus)

	shutdown = NewTimeWindow(*nextScheduledShutdown)

	if nextScheduledWakeup != nil {
		wakeup = NewTimeWindow(*nextScheduledWakeup)
	}

	return shutdown, wakeup
}

func (r *SleepCycleReconciler) GetSchedulesTime(sleepCycle corev1alpha1.SleepCycle, useStatus bool) (shutdown *time.Time, wakeup *time.Time) {

	shutdown = nil
	wakeup = nil

	if useStatus {
		if sleepCycle.Status.NextScheduledShutdownTime != nil {
			shutdown = &sleepCycle.Status.NextScheduledShutdownTime.Time
		} else {
			t := cronexpr.MustParse(sleepCycle.Spec.Shutdown).Next(time.Now())
			shutdown = &t
		}

		if sleepCycle.Status.NextScheduledWakeupTime != nil {
			wakeup = &sleepCycle.Status.NextScheduledWakeupTime.Time
		} else {
			wakeupCronExpression, err := cronexpr.Parse(sleepCycle.Spec.WakeUp)
			if err == nil {
				t := wakeupCronExpression.Next(time.Now())
				wakeup = &t
			}
		}
	} else {
		t := cronexpr.MustParse(sleepCycle.Spec.Shutdown).Next(time.Now())
		shutdown = &t
		wakeupCronExpression, err := cronexpr.Parse(sleepCycle.Spec.WakeUp)
		if err == nil {
			t := wakeupCronExpression.Next(time.Now())
			wakeup = &t
		}
	}

	return shutdown, wakeup
}
