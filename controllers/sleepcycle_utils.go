package controllers

import (
	"encoding/base64"
	corev1alpha1 "github.com/rekuberate-io/sleepcycles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"strings"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

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

//func (r *SleepCycleReconciler) generateSecureToken() (string, error) {
//	b := make([]byte, n)
//	rand.Read(b)
//
//	// Convert bytes to printable string
//	for i := 0; i < len(b); i++ {
//		b[i] = alphanumericCharacters[b[i]%byte(len(alphanumericCharacters))]
//	}
//
//	return string(b)
//}

//func generateRandomToken(min, max int) string {
//	n := rand.Intn(max-min+1) + min
//	chars := []rune("ABC???0123")
//
//	return generateRandomString(n, chars)
//}

func (r *SleepCycleReconciler) generateRandomString(n int) string {
	var characters = []rune(chars)
	var sb strings.Builder

	for i := 0; i < n; i++ {
		randomIndex := rand.Intn(len(characters))
		randomChar := characters[randomIndex]
		sb.WriteRune(randomChar)
	}

	return sb.String()
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
