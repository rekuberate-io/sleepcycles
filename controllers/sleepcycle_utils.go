package controllers

import (
	"encoding/base64"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"strings"
)

func (r *SleepCycleReconciler) hasLabel(obj *metav1.ObjectMeta, tag string) bool {
	val, ok := obj.GetLabels()[SleepCycleLabel]

	if ok && val == tag {
		return true
	}

	return false
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

func (r *SleepCycleReconciler) recordEvent(sleepCycle *corev1alpha1.SleepCycle, message string, isError bool) {
	eventType := corev1.EventTypeNormal
	reason := "SleepCycleOpSuccess"

	if isError {
		eventType = corev1.EventTypeWarning
		reason = "SleepCycleOpFailure"
	}

	r.Recorder.Event(sleepCycle, eventType, reason, strings.ToLower(message))
}
