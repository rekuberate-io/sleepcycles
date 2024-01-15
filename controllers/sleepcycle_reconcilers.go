package controllers

import (
	"context"
	"fmt"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	if len(deploymentList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("📚 Processing Deployments")

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
					err := r.ScaleDeployment(ctx, deployment, 0)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling Deployment failed", "deployment", deploymentFullName)
						r.RecordEvent(*sleepCycle, true, deploymentFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("🌙 Scaled Down Deployment", "deployment", deploymentFullName, "targetReplicas", 0)
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[deploymentFullName])

				if deployment.Status.Replicas != targetReplicas {
					err := r.ScaleDeployment(ctx, deployment, targetReplicas)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling Deployment failed", "deployment", deploymentFullName)
						r.RecordEvent(*sleepCycle, true, deploymentFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("☀️  Scaled Up Deployment", "deployment", deploymentFullName, "targetReplicas", targetReplicas)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SleepCycleReconciler) ReconcileCronJobs(ctx context.Context,
	req ctrl.Request,
	sleepCycle *corev1alpha1.SleepCycle,
	deepCopy *corev1alpha1.SleepCycle,
	op SleepCycleOperation,
) (ctrl.Result, error) {
	cronJobList := batchv1.CronJobList{}
	if err := r.List(ctx, &cronJobList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
		return ctrl.Result{}, err
	}

	if len(cronJobList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("🕑 Processing CronJobs")

	for _, cronJob := range cronJobList.Items {
		hasSleepCycle := r.isTagged(&cronJob.ObjectMeta, sleepCycle.Name)

		if hasSleepCycle {
			cronJobFullName := fmt.Sprintf("%v/%v", cronJob.Namespace, cronJob.Name)

			switch op {
			case Watch:
			case Shutdown:
				if !*cronJob.Spec.Suspend {
					err := r.SuspendCronJob(ctx, cronJob, true)
					if err != nil {
						r.logger.Error(err, "🛑️️ Suspending CronJob failed", "cronJob", cronJobFullName)
						r.RecordEvent(*sleepCycle, true, cronJobFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("🌙 Suspended CronJob", "cronJob", cronJobFullName)
				}
			case WakeUp:
				if *cronJob.Spec.Suspend {
					err := r.SuspendCronJob(ctx, cronJob, false)
					if err != nil {
						r.logger.Error(err, "🛑️️ Suspending CronJob failed", "cronJob", cronJobFullName)
						r.RecordEvent(*sleepCycle, true, cronJobFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("☀️  Enabled Cronjob", "cronJob", cronJobFullName)
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

	if len(statefulSetList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("📦 Processing StatefulSets")

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
					err := r.ScaleStatefulSet(ctx, statefulSet, 0)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling StatefulSet failed", "statefulSet", statefulSetFullName)
						r.RecordEvent(*sleepCycle, true, statefulSetFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("🌙 Scaled Down StatefulSet", "statefulSet", statefulSetFullName, "targetReplicas", 0)
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[statefulSetFullName])

				if statefulSet.Status.Replicas != targetReplicas {
					err := r.ScaleStatefulSet(ctx, statefulSet, targetReplicas)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling StatefulSet failed", "statefulSet", statefulSetFullName)
						r.RecordEvent(*sleepCycle, true, statefulSetFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("☀️  Scaled Up StatefulSet", "statefulSet", statefulSetFullName, "targetReplicas", targetReplicas)
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

	if len(hpaList.Items) == 0 {
		return ctrl.Result{}, nil
	}

	r.logger.Info("📈 Processing HorizontalPodAutoscalers")

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
					err := r.ScaleHorizontalPodAutoscaler(ctx, hpa, 1)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling HorizontalPodAutoscaler failed", "hpa", hpaFullName)
						r.RecordEvent(*sleepCycle, true, hpaFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("🌙 Scaled Down HorizontalPodAutoscaler", "hpa", hpaFullName, "maxReplicas", 1)
				}
			case WakeUp:
				targetReplicas := int32(deepCopy.Status.UsedBy[hpaFullName])

				if hpa.Spec.MaxReplicas != targetReplicas {
					err := r.ScaleHorizontalPodAutoscaler(ctx, hpa, targetReplicas)
					if err != nil {
						r.logger.Error(err, "🛑️ Scaling HorizontalPodAutoscaler failed", "hpa", hpaFullName)
						r.RecordEvent(*sleepCycle, true, hpaFullName, op)
						return ctrl.Result{}, err
					}

					r.logger.Info("☀️  Scaled Up HorizontalPodAutoscaler", "hpa", hpaFullName, "maxReplicas", targetReplicas)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}
