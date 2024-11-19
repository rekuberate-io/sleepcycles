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
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	SleepCycleLabel                            = "rekuberate.io/sleepcycle"
	TimeWindowToleranceInSeconds int           = 30
	requeueAfter                 time.Duration = 60 * time.Second
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
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error)

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
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete;escalate
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete;escalate
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete;escalate
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;escalate

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
	r.logger.Info("reconciling sleepcycle")

	nsks := "kube-system"
	if req.Namespace == nsks {
		r.logger.Info(fmt.Sprintf("setting sleepcycle schedule on resources in namespace %s is not supported", nsks))
		return ctrl.Result{}, nil
	}

	var original corev1alpha1.SleepCycle
	if err := r.Get(ctx, req.NamespacedName, &original); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		r.logger.Error(err, "unable to fetch sleepcycle")
		return ctrl.Result{}, err
	}

	err := r.reconcileRbac(ctx, &original)
	if err != nil {
		r.recordEvent(&original, fmt.Sprintf("unable to create rbac resources in %s", req.Namespace), false)
		return ctrl.Result{}, err
	}

	reconcilers := []runtimeObjectReconciler{r.ReconcileDeployments, r.ReconcileCronJobs, r.ReconcileStatefulSets, r.ReconcileHorizontalPodAutoscalers}
	var errors error
	provisioned := 0
	total := 0

	for _, reconciler := range reconcilers {
		p, t, err := reconciler(ctx, &original)
		if err != nil {
			errors = multierror.Append(errors, err)
		}

		provisioned += p
		total += t
	}

	if errors != nil {
		if merr, ok := errors.(*multierror.Error); ok {
			for _, rerr := range merr.Errors {
				r.logger.Error(rerr, "failed to reconcile")
			}
		}
	}

	err = r.UpdateStatus(ctx, &original, r.getStatusState(provisioned, total), []int{provisioned, total})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, errors
}

func (r *SleepCycleReconciler) UpdateStatus(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle, state string, targets []int) error {
	patch := client.MergeFrom(sleepcycle.DeepCopy())
	sleepcycle.Status.State = state
	sleepcycle.Status.Targets = fmt.Sprintf("%d/%d", targets[0], targets[1])

	err := r.Status().Patch(ctx, sleepcycle, patch)
	if err != nil {
		r.logger.Error(err, "unable to patch sleepcycle status")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SleepCycleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.SleepCycle{}, eventFilters).
		Complete(r)
}
