package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var (
	startingDeadlineSeconds int64 = 15
)

const (
	OwnedBy        = "rekuberate.io/owned-by"
	Target         = "rekuberate.io/target"
	TargetKind     = "rekuberate.io/target-kind"
	TargetTimezone = "rekuberate.io/target-tz"
	Replicas       = "rekuberate.io/replicas"
)

func (r *SleepCycleReconciler) getCronJob(ctx context.Context, objKey client.ObjectKey) (*batchv1.CronJob, error) {
	var job batchv1.CronJob
	if err := r.Get(ctx, objKey, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return &job, nil
}

func (r *SleepCycleReconciler) createCronJob(
	ctx context.Context,
	logger logr.Logger,
	sleepcycle *corev1alpha1.SleepCycle,
	cronObjectKey client.ObjectKey,
	targetKind string,
	targetMeta metav1.ObjectMeta,
	targetReplicas int32,
	isShutdownOp bool,
) (*batchv1.CronJob, error) {

	logger.Info("creating runner", "cronjob", cronObjectKey)
	backOffLimit := int32(0)

	schedule := sleepcycle.Spec.Shutdown
	tz := sleepcycle.Spec.ShutdownTimeZone
	suspend := !sleepcycle.Spec.Enabled

	if !isShutdownOp {
		schedule = *sleepcycle.Spec.WakeUp
		tz = sleepcycle.Spec.WakeupTimeZone
	}

	labels := make(map[string]string)
	labels[OwnedBy] = fmt.Sprintf("%s", sleepcycle.Name)
	labels[Target] = fmt.Sprintf("%s", targetMeta.Name)
	labels[TargetKind] = targetKind

	annotations := make(map[string]string)
	annotations[TargetTimezone] = *tz
	annotations[Replicas] = fmt.Sprint(targetReplicas)

	if targetReplicas == 0 {
		annotations[Replicas] = strconv.FormatInt(1, 10)
	}

	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cronObjectKey.Name,
			Namespace:   cronObjectKey.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.CronJobSpec{
			SuccessfulJobsHistoryLimit: &sleepcycle.Spec.SuccessfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &sleepcycle.Spec.FailedJobsHistoryLimit,
			Schedule:                   schedule,
			TimeZone:                   tz,
			StartingDeadlineSeconds:    &startingDeadlineSeconds,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			Suspend:                    &suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  cronObjectKey.Name,
									Image: sleepcycle.Spec.RunnerImage,
									Env: []v1.EnvVar{
										{
											Name: "MY_POD_NAME",
											ValueFrom: &v1.EnvVarSource{
												FieldRef: &v1.ObjectFieldSelector{
													FieldPath: "metadata.name",
												}},
										},
										{
											Name: "MY_POD_NAMESPACE",
											ValueFrom: &v1.EnvVarSource{
												FieldRef: &v1.ObjectFieldSelector{
													FieldPath: "metadata.namespace",
												}},
										},
										{
											Name:  "MY_CRONJOB_NAME",
											Value: cronObjectKey.Name,
										},
									},
								},
							},
							RestartPolicy:      v1.RestartPolicyOnFailure,
							ServiceAccountName: "rekuberate-runner",
						},
					},
					BackoffLimit: &backOffLimit,
				},
			},
		},
	}

	err := ctrl.SetControllerReference(sleepcycle, job, r.Scheme)
	if err != nil {
		logger.Error(err, "unable to set controller reference for runner", "cronjob", cronObjectKey.Name)
		return nil, err
	}

	err = r.Create(ctx, job)
	if err != nil {
		logger.Error(err, "unable to create runner", "cronjob", cronObjectKey.Name)
		return nil, err
	}

	return job, nil
}

func (r *SleepCycleReconciler) updateCronJob(
	ctx context.Context,
	logger logr.Logger,
	cronJob *batchv1.CronJob,
	schedule string,
	timezone string,
	suspend bool,
	replicas int32,
) error {
	deepCopy := cronJob.DeepCopy()
	deepCopy.Spec.Schedule = schedule
	*deepCopy.Spec.TimeZone = timezone
	*deepCopy.Spec.Suspend = suspend

	if replicas != 0 {
		deepCopy.Annotations[Replicas] = fmt.Sprint(replicas)
	}

	if err := r.Update(ctx, deepCopy); err != nil {
		logger.Error(err, "unable to update runner", "cronjob", cronJob.Name)
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) deleteCronJob(ctx context.Context, cronJob *batchv1.CronJob) error {
	if err := r.Delete(ctx, cronJob); err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) reconcileCronJob(
	ctx context.Context,
	logger logr.Logger,
	sleepcycle *corev1alpha1.SleepCycle,
	targetKind string,
	targetMeta metav1.ObjectMeta,
	targetReplicas int32,
	isShutdownOp bool,
) error {
	suffix := "shutdown"
	if !isShutdownOp {
		suffix = "wakeup"
	}

	cronObjectKey := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s-%s", sleepcycle.Name, targetMeta.Name, suffix),
		Namespace: sleepcycle.Namespace,
	}
	cronjob, err := r.getCronJob(ctx, cronObjectKey)
	if err != nil {
		logger.Error(err, "unable to fetch runner", "cronjob", cronObjectKey.Name)
		return err
	}

	if cronjob == nil {
		_, err := r.createCronJob(ctx, logger, sleepcycle, cronObjectKey, targetKind, targetMeta, targetReplicas, isShutdownOp)
		if err != nil {
			return err
		}
	}

	if cronjob != nil {
		if !isShutdownOp && sleepcycle.Spec.WakeUp == nil {
			err := r.deleteCronJob(ctx, cronjob)
			if err != nil {
				return err
			}
		}

		schedule := sleepcycle.Spec.Shutdown
		tz := sleepcycle.Spec.ShutdownTimeZone
		suspend := !sleepcycle.Spec.Enabled
		if !isShutdownOp {
			schedule = *sleepcycle.Spec.WakeUp
			tz = sleepcycle.Spec.WakeupTimeZone
		}

		err := r.updateCronJob(ctx, logger, cronjob, schedule, *tz, suspend, targetReplicas)
		if err != nil {
			logger.Error(err, "failed to update runner", "name", cronObjectKey.Name)
			return err
		}
	}

	return nil
}

func (r *SleepCycleReconciler) reconcile(
	ctx context.Context,
	logger logr.Logger,
	sleepcycle *corev1alpha1.SleepCycle,
	targetKind string,
	targetMeta metav1.ObjectMeta,
	targetReplicas int32,
) error {
	hasSleepCycle := r.hasLabel(&targetMeta, sleepcycle.Name)
	if hasSleepCycle {
		err := r.reconcileCronJob(ctx, logger, sleepcycle, targetKind, targetMeta, targetReplicas, true)
		if err != nil {
			return err
		}

		if sleepcycle.Spec.WakeUp != nil {
			err := r.reconcileCronJob(ctx, logger, sleepcycle, targetKind, targetMeta, targetReplicas, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
