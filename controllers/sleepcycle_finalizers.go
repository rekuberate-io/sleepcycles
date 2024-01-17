package controllers

import (
	"context"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SleepCycleReconciler) FinalizeDeployments(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
) (ctrl.Result, error) {
	deploymentList := appsv1.DeploymentList{}
	if err := r.List(ctx, &deploymentList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	if len(deploymentList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("🧽 Removing Labels from Deployments")

	for _, deployment := range deploymentList.Items {
		hasSleepCycle := r.hasLabel(&deployment.ObjectMeta, original.Name)
		if hasSleepCycle {
			delete(deployment.Labels, SleepCycleLabel)
			if err := r.Update(ctx, &deployment); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) FinalizeCronJobs(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
) (ctrl.Result, error) {
	cronJobList := batchv1.CronJobList{}
	if err := r.List(ctx, &cronJobList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	if len(cronJobList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("🧽 Removing Labels from CronJobs")

	for _, cronJob := range cronJobList.Items {
		hasSleepCycle := r.hasLabel(&cronJob.ObjectMeta, original.Name)
		if hasSleepCycle {
			delete(cronJob.Labels, SleepCycleLabel)
			if err := r.Update(ctx, &cronJob); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) FinalizeStatefulSets(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
) (ctrl.Result, error) {
	statefulSetList := appsv1.StatefulSetList{}
	if err := r.List(ctx, &statefulSetList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	if len(statefulSetList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("🧽 Removing Labels from StatefulSets")

	for _, statefulSet := range statefulSetList.Items {
		hasSleepCycle := r.hasLabel(&statefulSet.ObjectMeta, original.Name)
		if hasSleepCycle {
			delete(statefulSet.Labels, SleepCycleLabel)
			if err := r.Update(ctx, &statefulSet); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) FinalizeHorizontalPodAutoscalers(
	ctx context.Context,
	req ctrl.Request,
	original *corev1alpha1.SleepCycle,
) (ctrl.Result, error) {
	hpaList := autoscalingv1.HorizontalPodAutoscalerList{}
	if err := r.List(ctx, &hpaList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	if len(hpaList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("🧽 Removing Labels from HorizontalPodAutoscalers")

	for _, hpa := range hpaList.Items {
		hasSleepCycle := r.hasLabel(&hpa.ObjectMeta, original.Name)
		if hasSleepCycle {
			delete(hpa.Labels, SleepCycleLabel)
			if err := r.Update(ctx, &hpa); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}
