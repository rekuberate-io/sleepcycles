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
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
	"time"

	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	SleepCycleLabel                      = "rekuberate.io/sleepcycle"
	SleepCycleFinalizer                  = "sleepcycle.core.rekuberate.io/finalizer"
	TimeWindowToleranceInSeconds  int    = 30
	SleepCycleStatusUpdateFailure string = "failed to update SleepCycle Status"
	SleepCycleFinalizerFailure    string = "finalizer failed"
)

// SleepCycleReconciler reconciles a SleepCycle object
type SleepCycleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	logger   logr.Logger
	Recorder record.EventRecorder
}

type runtimeObjectReconciler func(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
	desired *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error)

type runtimeObjectFinalizer func(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
) (ctrl.Result, error)

var (
	eventFilters = builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// We only need to check generation changes here, because it is only
			// updated on spec changes. On the other hand RevisionVersion
			// changes also on status changes. We want to omit reconciliation
			// for status updates.
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// DeleteStateUnknown evaluates to false only if the object
			// has been confirmed as deleted by the api server.
			return !e.DeleteStateUnknown
		},
	})
)

//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.rekuberate.io,resources=sleepcycles/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the SleepCycle object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SleepCycleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.Log.WithValues("namespace", req.Namespace, "sleepcycle", req.Name)

	var original corev1alpha1.SleepCycle
	if err := r.Get(ctx, req.NamespacedName, &original); err != nil {
		if apierrors.IsNotFound(err) {
			//r.logger.Error(err, "🛑️ unable to find SleepCycle")
			return ctrl.Result{}, nil
		}

		r.logger.Error(err, "🛑 unable to fetch SleepCycle")
		return ctrl.Result{}, err
	}

	sleepCycleFullName := fmt.Sprintf("%v/%v", original.Namespace, original.Name)

	//TODO: Bug finalizer creates a deadlock when removing or upgrading

	//if original.ObjectMeta.DeletionTimestamp.IsZero() {
	//	// The object is not being deleted, so if it does not have our finalizer,
	//	// then lets add the finalizer and update the object.
	//	if !containsString(original.ObjectMeta.Finalizers, SleepCycleFinalizer) {
	//		original.ObjectMeta.Finalizers = append(original.ObjectMeta.Finalizers, SleepCycleFinalizer)
	//		if err := r.Update(ctx, &original); err != nil {
	//			r.logger.Error(err, fmt.Sprintf("🛑️ %s", SleepCycleStatusUpdateFailure), "sleepcycle", sleepCycleFullName)
	//			r.Recorder.Event(&original, corev1.EventTypeWarning, "SleepCycleStatus", strings.ToLower(SleepCycleStatusUpdateFailure))
	//			return ctrl.Result{}, err
	//		}
	//	}
	//} else {
	//	// The object is being deleted
	//	if containsString(original.ObjectMeta.Finalizers, SleepCycleFinalizer) {
	//		// our finalizer is present, so lets handle our external dependency
	//		finalizers := []runtimeObjectFinalizer{r.FinalizeDeployments, r.FinalizeCronJobs, r.FinalizeStatefulSets, r.FinalizeHorizontalPodAutoscalers}
	//		for _, finalizer := range finalizers {
	//			result, err := finalizer(ctx, req, &original)
	//			if err != nil {
	//				r.logger.Error(err, fmt.Sprintf("🛑️ %s", SleepCycleFinalizerFailure), "sleepcycle", sleepCycleFullName)
	//				r.Recorder.Event(&original, corev1.EventTypeWarning, "SleepCycleFinalizerFailure", fmt.Sprintf(
	//					"%s: %s",
	//					SleepCycleFinalizerFailure,
	//					err.Error(),
	//				))
	//
	//				return result, err
	//			}
	//		}
	//
	//		// remove our finalizer from the list and update it.
	//		original.ObjectMeta.Finalizers = removeString(original.ObjectMeta.Finalizers, SleepCycleFinalizer)
	//		if err := r.Update(ctx, &original); err != nil {
	//			r.logger.Error(err, fmt.Sprintf("🛑️ %s", SleepCycleStatusUpdateFailure), "sleepcycle", sleepCycleFullName)
	//			r.Recorder.Event(&original, corev1.EventTypeWarning, "SleepCycleStatus", strings.ToLower(SleepCycleStatusUpdateFailure))
	//			return ctrl.Result{}, err
	//		}
	//	}
	//
	//	// Our finalizer has finished, so the reconciler can do nothing.
	//	return reconcile.Result{}, nil
	//}

	if !original.Spec.Enabled {
		return ctrl.Result{}, nil
	}

	currentOperation := r.getCurrentScheduledOperation(original)

	desired := *original.DeepCopy()
	desired.Status.LastRunOperation = currentOperation.String()
	if desired.Status.UsedBy == nil {
		usedBy := make(map[string]int)
		desired.Status.UsedBy = usedBy
	}

	r.logger = r.logger.WithValues("op", currentOperation.String())

	if currentOperation != Watch {
		reconcilers := []runtimeObjectReconciler{r.ReconcileDeployments, r.ReconcileCronJobs, r.ReconcileStatefulSets, r.ReconcileHorizontalPodAutoscalers}

		for _, reconciler := range reconcilers {
			result, err := reconciler(ctx, req, &original, &desired, currentOperation)
			if err != nil {
				if currentOperation != Watch {
					desired.Status.LastRunTime = &metav1.Time{Time: time.Now()}
					desired.Status.LastRunWasSuccessful = false
				}

				if err := r.Status().Update(ctx, &desired); err != nil {
					r.logger.Error(err, fmt.Sprintf("🛑️ %s", SleepCycleStatusUpdateFailure), "sleepcycle", sleepCycleFullName)
					r.Recorder.Event(&original, corev1.EventTypeWarning, "SleepCycleStatus", strings.ToLower(SleepCycleStatusUpdateFailure))
					return ctrl.Result{}, err
				}

				return result, err
			}
		}

		desired.Status.LastRunTime = &metav1.Time{Time: time.Now()}
		desired.Status.LastRunWasSuccessful = true
	}

	nextScheduledShutdown, nextScheduledWakeup := r.getSchedulesTime(original, false)
	if nextScheduledWakeup != nil {
		tz := r.getTimeZone(original.Spec.WakeupTimeZone)
		t := nextScheduledWakeup.In(tz)
		desired.Status.NextScheduledWakeupTime = &metav1.Time{Time: t}
	} else {
		desired.Status.NextScheduledWakeupTime = nil
	}
	tz := r.getTimeZone(original.Spec.ShutdownTimeZone)
	t := nextScheduledShutdown.In(tz)
	desired.Status.NextScheduledShutdownTime = &metav1.Time{Time: t}

	if err := r.Status().Update(ctx, &desired); err != nil {
		r.logger.Error(err, fmt.Sprintf("🛑️ %s", SleepCycleStatusUpdateFailure), "sleepcycle", sleepCycleFullName)
		r.Recorder.Event(&original, corev1.EventTypeWarning, "SleepCycleStatus", strings.ToLower(SleepCycleStatusUpdateFailure))
		return ctrl.Result{}, err
	}

	nextOperation, requeueAfter := r.getNextScheduledOperation(original, &currentOperation)

	r.logger.Info("Requeue", "next-op", nextOperation.String(), "after", requeueAfter)
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
		For(&corev1alpha1.SleepCycle{}, eventFilters).
		Complete(r)
}

func (r *SleepCycleReconciler) RecordEvent(sleepCycle corev1alpha1.SleepCycle, isError bool, namespacedName string, operation SleepCycleOperation, extra ...string) {
	eventType := corev1.EventTypeNormal
	reason := "SleepCycleOpSuccess"
	message := fmt.Sprintf("%s on %s succeeded", operation.String(), namespacedName)

	if isError {
		eventType = corev1.EventTypeWarning
		reason = "SleepCycleOpFailure"
		message = fmt.Sprintf("%s on %s failed", operation.String(), namespacedName)
	}

	for _, s := range extra {
		message = message + ". " + s
	}

	r.Recorder.Event(&sleepCycle, eventType, reason, strings.ToLower(message))
}
