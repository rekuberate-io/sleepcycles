package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

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

func (r *SleepCycleReconciler) generateSecureRandomString(length int) (string, error) {
	result := make([]byte, length)
	_, err := rand.Read(result)
	if err != nil {
		return "", err
	}

	for i := range result {
		result[i] = letters[int(result[i])%len(letters)]
	}
	return string(result), nil
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

// MergeLabels merges two given labels.
func MergeLabels(l ...map[string]string) map[string]string {
	res := make(map[string]string)

	for _, v := range l {
		for lKey, lValue := range v {
			res[lKey] = lValue
		}
	}
	return res
}

func MergeAnnotations(annotations ...map[string]string) map[string]string {
	rtn := make(map[string]string)
	for _, a := range annotations {
		for k, v := range a {
			rtn[k] = v
		}
	}

	return rtn
}
