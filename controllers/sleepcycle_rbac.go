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

func (r *SleepCycleReconciler) reconcileRbac(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) error {
	perr := fmt.Errorf("unable to create rbac resources")
	ready, err := r.checkRbac(ctx, sleepcycle)
	if err != nil {
		return err
	}

	if ready {
		return nil
	}

	err = r.deleteRbac(ctx, sleepcycle)
	if err != nil {
		return err
	}

	account, err := r.createServiceAccount(ctx, sleepcycle)
	if err != nil {
		return errors.Wrap(err, perr.Error())
	}

	_, err = r.createSecret(ctx, account)
	if err != nil {
		return errors.Wrap(err, perr.Error())
	}

	role, err := r.createRole(ctx, account)
	if err != nil {
		return errors.Wrap(err, perr.Error())
	}

	_, err = r.createRoleBinding(ctx, role)
	if err != nil {
		return errors.Wrap(err, perr.Error())
	}

	r.recordEvent(sleepcycle, fmt.Sprintf("created rbac resources in %s", sleepcycle.Namespace), false)
	return nil
}

func (r *SleepCycleReconciler) checkRbac(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) (bool, error) {
	ready := true

	saok := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      serviceAccountName,
	}
	var sa v1.ServiceAccount
	if err := r.Get(ctx, saok, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			ready = false
		} else {
			r.logger.Error(err, "unable to fetch service account")
			return false, err
		}
	}

	var ro rbacv1.Role
	rook := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      fmt.Sprintf("%s-role", serviceAccountName),
	}
	if err := r.Get(ctx, rook, &ro); err != nil {
		if apierrors.IsNotFound(err) {
			ready = false
		} else {
			r.logger.Error(err, "unable to fetch role")
			return false, err
		}
	}

	var rb rbacv1.RoleBinding
	rbok := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      fmt.Sprintf("%s-rolebinding", serviceAccountName),
	}
	if err := r.Get(ctx, rbok, &rb); err != nil {
		if apierrors.IsNotFound(err) {
			ready = false
		} else {
			r.logger.Error(err, "unable to fetch role binding")
			return false, err
		}
	}

	return ready, nil
}

func (r *SleepCycleReconciler) deleteRbac(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) error {
	saok := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      serviceAccountName,
	}
	var sa v1.ServiceAccount
	if err := r.Get(ctx, saok, &sa); err != nil {
		if !apierrors.IsNotFound(err) {
			r.logger.Error(err, "unable to fetch service account")
			return err
		}

		return nil
	}

	err := r.Delete(ctx, &sa)
	if err != nil {
		return err
	}

	return nil
}

func (r *SleepCycleReconciler) createServiceAccount(ctx context.Context, sleepcycle *corev1alpha1.SleepCycle) (*v1.ServiceAccount, error) {
	objectKey := client.ObjectKey{
		Namespace: sleepcycle.Namespace,
		Name:      serviceAccountName,
	}

	r.logger.Info("creating service account", "account", serviceAccountName)
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectKey.Name,
			Namespace: objectKey.Namespace,
		},
	}

	err := ctrl.SetControllerReference(sleepcycle, sa, r.Scheme)
	if err != nil {
		return nil, err
	}

	err = r.Create(ctx, sa)
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func (r *SleepCycleReconciler) createSecret(ctx context.Context, serviceAccount *v1.ServiceAccount) (*v1.Secret, error) {
	r.logger.Info("creating secret", "secret", fmt.Sprintf("%s-secret", serviceAccountName))

	token, err := r.generateToken()
	if err != nil {
		return nil, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", serviceAccountName),
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

	err = ctrl.SetControllerReference(serviceAccount, secret, r.Scheme)
	if err != nil {
		return nil, err
	}

	err = r.Create(ctx, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *SleepCycleReconciler) createRole(ctx context.Context, serviceAccount *v1.ServiceAccount) (*rbacv1.Role, error) {
	r.logger.Info("creating role", "role", fmt.Sprintf("%s-role", serviceAccountName))

	roleName := fmt.Sprintf("%s-role", serviceAccountName)
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

	err := ctrl.SetControllerReference(serviceAccount, role, r.Scheme)
	if err != nil {
		return nil, err
	}

	err = r.Create(ctx, role)
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (r *SleepCycleReconciler) createRoleBinding(ctx context.Context, role *rbacv1.Role) (*rbacv1.RoleBinding, error) {
	r.logger.Info("creating role binding", "role", fmt.Sprintf("%s-rolebinding", serviceAccountName))
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rolebinding", serviceAccountName),
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

	err := ctrl.SetControllerReference(role, roleBinding, r.Scheme)
	if err != nil {
		return nil, err
	}

	err = r.Create(ctx, roleBinding)
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}
