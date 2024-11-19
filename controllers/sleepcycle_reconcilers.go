package controllers

import (
	"context"
	"github.com/hashicorp/go-multierror"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
)

func (r *SleepCycleReconciler) ReconcileDeployments(
	ctx context.Context,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	deploymentList := appsv1.DeploymentList{}
	if err := r.List(
		ctx,
		&deploymentList,
		r.getListOptions(sleepcycle.Namespace, sleepcycle.Name)...,
	); err != nil {
		return 0, 0, err
	}

	var total, provisioned int
	if total = len(deploymentList.Items); total == 0 {
		return 0, 0, nil
	}
	provisioned = total

	var errors error
	for _, deployment := range deploymentList.Items {
		logger := r.logger.WithValues("deployment", deployment.Name)

		kind := deployment.TypeMeta.Kind
		meta := deployment.ObjectMeta
		replicas := *deployment.Spec.Replicas

		err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
		if err != nil {
			provisioned -= 1
			errors = multierror.Append(errors, err)
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileCronJobs(
	ctx context.Context,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	cronJobList := batchv1.CronJobList{}
	if err := r.List(
		ctx,
		&cronJobList,
		r.getListOptions(sleepcycle.Namespace, sleepcycle.Name)...,
	); err != nil {
		return 0, 0, err
	}

	var total, provisioned int
	if total = len(cronJobList.Items); total == 0 {
		return 0, 0, nil
	}
	provisioned = total

	var errors error
	for _, cronjob := range cronJobList.Items {
		logger := r.logger.WithValues("cronjob", cronjob.Name)

		kind := cronjob.TypeMeta.Kind
		meta := cronjob.ObjectMeta

		replicas := int32(1)
		if *cronjob.Spec.Suspend {
			replicas = int32(0)
		}

		err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
		if err != nil {
			provisioned -= 1
			errors = multierror.Append(errors, err)
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileStatefulSets(
	ctx context.Context,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	statefulSetList := appsv1.StatefulSetList{}
	if err := r.List(
		ctx,
		&statefulSetList,
		r.getListOptions(sleepcycle.Namespace, sleepcycle.Name)...,
	); err != nil {
		return 0, 0, err
	}

	var total, provisioned int
	if total = len(statefulSetList.Items); total == 0 {
		return 0, 0, nil
	}
	provisioned = total

	var errors error
	for _, statefulSet := range statefulSetList.Items {
		logger := r.logger.WithValues("statefulset", statefulSet.Name)

		kind := statefulSet.TypeMeta.Kind
		meta := statefulSet.ObjectMeta
		replicas := *statefulSet.Spec.Replicas

		err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
		if err != nil {
			provisioned -= 1
			errors = multierror.Append(errors, err)
		}
	}

	return provisioned, total, errors
}

func (r *SleepCycleReconciler) ReconcileHorizontalPodAutoscalers(
	ctx context.Context,
	sleepcycle *corev1alpha1.SleepCycle,
) (int, int, error) {
	hpaList := autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.List(
		ctx,
		&hpaList,
		r.getListOptions(sleepcycle.Namespace, sleepcycle.Name)...,
	); err != nil {
		return 0, 0, err
	}

	var total, provisioned int
	if total = len(hpaList.Items); total == 0 {
		return 0, 0, nil
	}
	provisioned = total

	var errors error
	for _, hpa := range hpaList.Items {
		logger := r.logger.WithValues("hpa", hpa.Name)

		kind := hpa.TypeMeta.Kind
		meta := hpa.ObjectMeta
		replicas := hpa.Spec.MaxReplicas

		err := r.reconcile(ctx, logger, sleepcycle, kind, meta, replicas)
		if err != nil {
			provisioned -= 1
			errors = multierror.Append(errors, err)
		}
	}

	return provisioned, total, errors
}
