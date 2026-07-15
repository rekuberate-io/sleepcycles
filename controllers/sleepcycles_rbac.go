package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serviceAccountName = "sleecycles-runner"
)

func (r *SleepCycleReconciler) reconcileRbac(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) (bool, error) {
	perr := fmt.Errorf("unable to create rbac resources")

	account, requeue, err := r.ensureServiceAccount(ctx, sleepcycle)
	if err != nil {
		return false, errors.Wrap(err, perr.Error())
	}
	if requeue {
		return true, nil
	}

	_, requeue, err = r.ensureSecret(ctx, account)
	if err != nil {
		return false, errors.Wrap(err, perr.Error())
	}
	if requeue {
		return true, nil
	}

	role, requeue, err := r.ensureRole(ctx, account)
	if err != nil {
		return false, errors.Wrap(err, perr.Error())
	}
	if requeue {
		return true, nil
	}

	_, requeue, err = r.ensureRoleBinding(ctx, role)
	if err != nil {
		return false, errors.Wrap(err, perr.Error())
	}
	if requeue {
		return true, nil
	}

	r.recordEvent(sleepcycle, fmt.Sprintf("reconciled rbac resources in %s", sleepcycle.Namespace), false)
	return false, nil
}

func (r *SleepCycleReconciler) ensureServiceAccount(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) (*v1.ServiceAccount, bool, error) {
	objectKey := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      serviceAccountName,
	}

	var existing v1.ServiceAccount
	err := r.Get(ctx, objectKey, &existing)
	if err == nil {
		if !existing.DeletionTimestamp.IsZero() {
			r.logger.Info("service account is terminating, waiting before recreating", "account", serviceAccountName)
			return nil, true, nil
		}
		return &existing, false, nil
	}
	if !apierrors.IsNotFound(err) {
		r.logger.Error(err, "unable to fetch service account")
		return nil, false, err
	}

	r.logger.Info("creating service account", "account", serviceAccountName)
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectKey.Name,
			Namespace: objectKey.Namespace,
		},
	}

	if err := ctrl.SetControllerReference(sleepcycle, sa, r.Scheme); err != nil {
		return nil, false, err
	}

	if err := r.Create(ctx, sa); err != nil {
		// Someone (or a previous reconcile) created it concurrently, or an old
		// instance is still terminating. Requeue instead of erroring out.
		if apierrors.IsAlreadyExists(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	return sa, false, nil
}

func (r *SleepCycleReconciler) ensureSecret(ctx context.Context, serviceAccount *v1.ServiceAccount) (*v1.Secret, bool, error) {
	secretName := fmt.Sprintf("%s-secret", serviceAccountName)
	objectKey := client.ObjectKey{
		Namespace: serviceAccount.Namespace,
		Name:      secretName,
	}

	var existing v1.Secret
	err := r.Get(ctx, objectKey, &existing)
	if err == nil {
		if !existing.DeletionTimestamp.IsZero() {
			r.logger.Info("secret is terminating, waiting before recreating", "secret", secretName)
			return nil, true, nil
		}
		return &existing, false, nil
	}
	if !apierrors.IsNotFound(err) {
		r.logger.Error(err, "unable to fetch secret")
		return nil, false, err
	}

	r.logger.Info("creating secret", "secret", secretName)
	token, err := r.generateToken()
	if err != nil {
		return nil, false, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": serviceAccountName,
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}

	if err := ctrl.SetControllerReference(serviceAccount, secret, r.Scheme); err != nil {
		return nil, false, err
	}

	if err := r.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	return secret, false, nil
}

func (r *SleepCycleReconciler) ensureRole(ctx context.Context, serviceAccount *v1.ServiceAccount) (*rbacv1.Role, bool, error) {
	roleName := fmt.Sprintf("%s-role", serviceAccountName)
	objectKey := client.ObjectKey{
		Namespace: serviceAccount.Namespace,
		Name:      roleName,
	}

	var existing rbacv1.Role
	err := r.Get(ctx, objectKey, &existing)
	if err == nil {
		if !existing.DeletionTimestamp.IsZero() {
			r.logger.Info("role is terminating, waiting before recreating", "role", roleName)
			return nil, true, nil
		}
		return &existing, false, nil
	}
	if !apierrors.IsNotFound(err) {
		r.logger.Error(err, "unable to fetch role")
		return nil, false, err
	}

	r.logger.Info("creating role", "role", roleName)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: serviceAccount.Namespace,
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

	if err := ctrl.SetControllerReference(serviceAccount, role, r.Scheme); err != nil {
		return nil, false, err
	}

	if err := r.Create(ctx, role); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	return role, false, nil
}

func (r *SleepCycleReconciler) ensureRoleBinding(ctx context.Context, role *rbacv1.Role) (*rbacv1.RoleBinding, bool, error) {
	roleBindingName := fmt.Sprintf("%s-rolebinding", serviceAccountName)
	objectKey := client.ObjectKey{
		Namespace: role.Namespace,
		Name:      roleBindingName,
	}

	var existing rbacv1.RoleBinding
	err := r.Get(ctx, objectKey, &existing)
	if err == nil {
		if !existing.DeletionTimestamp.IsZero() {
			r.logger.Info("role binding is terminating, waiting before recreating", "rolebinding", roleBindingName)
			return nil, true, nil
		}
		return &existing, false, nil
	}
	if !apierrors.IsNotFound(err) {
		r.logger.Error(err, "unable to fetch role binding")
		return nil, false, err
	}

	r.logger.Info("creating role binding", "rolebinding", roleBindingName)
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: role.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: role.Namespace,
			},
		},
	}

	if err := ctrl.SetControllerReference(role, roleBinding, r.Scheme); err != nil {
		return nil, false, err
	}

	if err := r.Create(ctx, roleBinding); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	return roleBinding, false, nil
}
