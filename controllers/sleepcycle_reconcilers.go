package controllers

import (
	"context"

	"github.com/hashicorp/go-multierror"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	UsedByLabelKey = "(%v) %v/%v"
)

func (r *SleepCycleReconciler) ReconcileDeployments(
	ctx context.Context,
	req ctrl.Request,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	provisioned := 0
	total := 0

	deploymentList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deploymentList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return 0, 0, err
	}

	if len(deploymentList.Items) == 0 {
		return 0, 0, nil
	}

	var errors error
	for _, deployment := range deploymentList.Items {
		logger := r.logger.WithValues("deployment", deployment.Name)

		kind := deployment.TypeMeta.Kind
		meta := deployment.ObjectMeta
		replicas := *deployment.Spec.Replicas

		hasSleepCycle := r.hasLabel(&meta, sleepcycle.Name)
		if hasSleepCycle {
			total += 1
			provisioned += 1

			err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
			if err != nil {
				provisioned -= 1
				errors = multierror.Append(errors, err)
			}
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileCronJobs(
	ctx context.Context,
	req ctrl.Request,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	provisioned := 0
	total := 0

	cronJobList := batchv1.CronJobList{}
	if err := r.List(ctx, &cronJobList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return 0, 0, err
	}

	if len(cronJobList.Items) == 0 {
		return 0, 0, nil
	}

	var errors error
	for _, cronjob := range cronJobList.Items {
		logger := r.logger.WithValues("cronjob", cronjob.Name)

		kind := cronjob.TypeMeta.Kind
		meta := cronjob.ObjectMeta

		replicas := int32(1)
		if *cronjob.Spec.Suspend {
			replicas = int32(0)
		}

		hasSleepCycle := r.hasLabel(&meta, sleepcycle.Name)
		if hasSleepCycle {
			total += 1
			provisioned += 1

			err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
			if err != nil {
				provisioned -= 1
				errors = multierror.Append(errors, err)
			}
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileStatefulSets(
	ctx context.Context,
	req ctrl.Request,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	provisioned := 0
	total := 0

	statefulSetList := appsv1.StatefulSetList{}
	if err := r.List(ctx, &statefulSetList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return 0, 0, err
	}

	if len(statefulSetList.Items) == 0 {
		return 0, 0, nil
	}

	var errors error
	for _, statefulSet := range statefulSetList.Items {
		logger := r.logger.WithValues("statefulset", statefulSet.Name)

		kind := statefulSet.TypeMeta.Kind
		meta := statefulSet.ObjectMeta
		replicas := *statefulSet.Spec.Replicas

		hasSleepCycle := r.hasLabel(&meta, sleepcycle.Name)
		if hasSleepCycle {
			total += 1
			provisioned += 1

			err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
			if err != nil {
				provisioned -= 1
				errors = multierror.Append(errors, err)
			}
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileHorizontalPodAutoscalers(
	ctx context.Context,
	req ctrl.Request,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	provisioned := 0
	total := 0

	hpaList := autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.List(ctx, &hpaList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return 0, 0, err
	}

	if len(hpaList.Items) == 0 {
		return 0, 0, nil
	}

	var errors error
	for _, hpa := range hpaList.Items {
		logger := r.logger.WithValues("hpa", hpa.Name)

		kind := hpa.TypeMeta.Kind
		meta := hpa.ObjectMeta
		replicas := hpa.Spec.MaxReplicas

		hasSleepCycle := r.hasLabel(&meta, sleepcycle.Name)
		if hasSleepCycle {
			total += 1
			provisioned += 1

			err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
			if err != nil {
				provisioned -= 1
				errors = multierror.Append(errors, err)
			}
		}
	}

	return provisioned, total, errors
}
