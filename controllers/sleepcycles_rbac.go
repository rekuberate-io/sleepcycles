package controllers

import (
	"context"
	"fmt"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serviceAccountName = "sleecycles-runner"
)

func (r *SleepCycleReconciler) reconcileRbac(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) error {
	createServiceAccount := false
	serviceAccountObjectKey := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      serviceAccountName,
	}
	var serviceAccount v1.ServiceAccount
	if err := r.Get(ctx, serviceAccountObjectKey, &serviceAccount); err != nil {
		if apierrors.IsNotFound(err) {
			createServiceAccount = true
		} else {
			r.logger.Error(err, "unable to fetch service account")
			return err
		}
	}

	if !createServiceAccount {
		return nil
	}

	r.logger.Info("creating service account", "account", serviceAccountName)
	newServiceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountObjectKey.Name,
			Namespace: serviceAccountObjectKey.Namespace,
		},
	}
	err := r.Create(ctx, newServiceAccount)
	if err != nil {
		return err
	}

	token, err := r.generateToken()
	if err != nil {
		return err
	}

	r.logger.Info("creating secret", "secret", fmt.Sprintf("%s-secret", serviceAccountName))
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", serviceAccountName),
			Namespace: sleepcycle.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": serviceAccountName,
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}

	err = r.Create(ctx, secret)
	if err != nil {
		return err
	}

	r.logger.Info("creating role", "role", fmt.Sprintf("%s-role", serviceAccountName))
	roleName := fmt.Sprintf("%s-role", serviceAccountName)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: sleepcycle.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "replicasets", "statefulsets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"cronjobs"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	err = r.Create(ctx, role)
	if err != nil {
		return err
	}

	r.logger.Info("creating role binding", "role", fmt.Sprintf("%s-rolebinding", serviceAccountName))
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rolebinding", serviceAccountName),
			Namespace: sleepcycle.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: sleepcycle.Namespace,
			},
		},
	}

	err = r.Create(ctx, roleBinding)
	if err != nil {
		return err
	}

	r.recordEvent(sleepcycle, fmt.Sprintf("created rbac resources in %s", sleepcycle.Namespace), false)
	return nil
}
