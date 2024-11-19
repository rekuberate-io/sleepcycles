package controllers

import (
	"crypto/rand"
	"encoding/base64"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func (r *SleepCycleReconciler) getListOptions(namespace, name string) []client.ListOption {
	labelSelector := map[string]string{
		SleepCycleLabel: name,
	}
	listOptions := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelector),
	}

	return listOptions
}

func (r *SleepCycleReconciler) generateToken() (string, error) {
	token := make([]byte, 256)
	_, err := rand.Read(token)
	if err != nil {
		r.logger.Error(err, "error while generating the secret token")
		return "", err
	}

	base64EncodedToken := base64.StdEncoding.EncodeToString(token)
	return base64EncodedToken, nil
}

func (r *SleepCycleReconciler) generateSecureRandomString(length int) (string, error) {
	result := make([]byte, length)
	_, err := rand.Read(result)
	if err != nil {
		r.logger.Error(err, "error while generating a random string")
		return "", err
	}

	for i := range result {
		result[i] = letters[int(result[i])%len(letters)]
	}
	return string(result), nil
}

func (r *SleepCycleReconciler) getStatusState(provisioned, total int) (state string) {
	state = "Ready"
	if provisioned != 0 && provisioned < total {
		state = "Warning"
	} else if provisioned == 0 && total != 0 {
		state = "NotReady"
	}

	return state
}

func (r *SleepCycleReconciler) recordEvent(sleepCycle *corev1alpha1.SleepCycle, message string, isError bool) {
	eventType := corev1.EventTypeNormal
	reason := "SuccessfulSleepCycleReconcile"

	if isError {
		eventType = corev1.EventTypeWarning
		reason = "FailedSleepCycleReconcile"
	}

	r.Recorder.Event(sleepCycle, eventType, reason, strings.ToLower(message))
}
