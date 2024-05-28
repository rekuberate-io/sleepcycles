package controllers

import (
	"encoding/base64"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
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
