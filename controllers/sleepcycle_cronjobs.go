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
)

var (
	startingDeadlineSeconds int64 = 15
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
	objKey client.ObjectKey,
	isShutdownOp bool,
) (*batchv1.CronJob, error) {

	backOffLimit := int32(0)
	schedule := sleepcycle.Spec.Shutdown
	tz := sleepcycle.Spec.ShutdownTimeZone
	suspend := !sleepcycle.Spec.Enabled

	if !isShutdownOp {
		schedule = *sleepcycle.Spec.WakeUp
		tz = sleepcycle.Spec.WakeupTimeZone
	}

	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                schedule,
			TimeZone:                tz,
			StartingDeadlineSeconds: &startingDeadlineSeconds,
			ConcurrencyPolicy:       batchv1.ForbidConcurrent,
			Suspend:                 &suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:    objKey.Name,
									Image:   "ubuntu:latest",
									Command: []string{"ls", "-aRil"},
								},
							},
							RestartPolicy: v1.RestartPolicyOnFailure,
						},
					},
					BackoffLimit: &backOffLimit,
				},
			},
		},
	}

	err := ctrl.SetControllerReference(sleepcycle, job, r.Scheme)
	if err != nil {
		logger.Error(err, "unable to set controller reference", "cronjob", objKey.Name)
		return nil, err
	}

	err = r.Create(ctx, job)
	if err != nil {
		logger.Error(err, "unable to create internal cronjob", "cronjob", objKey.Name)
		return nil, err
	}

	return job, nil
}

func (r *SleepCycleReconciler) updateCronJob(ctx context.Context, cronJob *batchv1.CronJob, schedule string, timezone string, suspend bool) error {
	deepCopy := cronJob.DeepCopy()
	deepCopy.Spec.Schedule = schedule
	*deepCopy.Spec.TimeZone = timezone
	*deepCopy.Spec.Suspend = suspend

	if err := r.Update(ctx, deepCopy); err != nil {
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

func (r *SleepCycleReconciler) reconcileCronJob(ctx context.Context, logger logr.Logger, sleepcycle *corev1alpha1.SleepCycle, objectMetadata ctrl.ObjectMeta, isShutdownOp bool) error {
	suffix := "shutdown"
	if !isShutdownOp {
		suffix = "wakeup"
	}

	objectKey := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s-%s", sleepcycle.Name, objectMetadata.Name, suffix),
		Namespace: sleepcycle.Namespace,
	}
	cronjob, err := r.getCronJob(ctx, objectKey)
	if err != nil {
		logger.Error(err, "unable to fetch internal cronjob", "cronjob", objectKey.Name)
		return err
	}

	if cronjob == nil {
		_, err := r.createCronJob(ctx, logger, sleepcycle, objectKey, isShutdownOp)
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

		if cronjob.Spec.Suspend != &suspend || cronjob.Spec.Schedule != schedule || cronjob.Spec.TimeZone != tz {
			err := r.updateCronJob(ctx, cronjob, schedule, *tz, suspend)
			if err != nil {
				logger.Error(err, "failed to update internal cronjob", "name", objectKey.Name)
				return err
			}
		}
	}

	return nil
}

func (r *SleepCycleReconciler) reconcile(ctx context.Context, logger logr.Logger, sleepcycle *corev1alpha1.SleepCycle, objectMetadata ctrl.ObjectMeta) error {
	hasSleepCycle := r.hasLabel(&objectMetadata, sleepcycle.Name)
	if hasSleepCycle {
		err := r.reconcileCronJob(ctx, logger, sleepcycle, objectMetadata, true)
		if err != nil {
			return err
		}

		if sleepcycle.Spec.WakeUp != nil {
			err := r.reconcileCronJob(ctx, logger, sleepcycle, objectMetadata, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
