package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	logger logr.Logger
	config *rest.Config
	ctx    context.Context

	envVarErr       = "environment variable %s not found"
	podEnvVar       = "MY_POD_NAME"
	namespaceEnvVar = "MY_POD_NAMESPACE"
)

func init() {
	opts := zap.Options{
		Development:     true,
		TimeEncoder:     zapcore.ISO8601TimeEncoder,
		StacktraceLevel: zapcore.DPanicLevel,
	}
	opts.BindFlags(flag.CommandLine)

	logger = zap.New(zap.UseFlagOptions(&opts))
	config = ctrl.GetConfigOrDie()
	ctx = context.Background()
}

func main() {
	pod, ok := os.LookupEnv(podEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, podEnvVar), "failed to load environment variable")
		os.Exit(78)
	}

	namespace, ok := os.LookupEnv(namespaceEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, namespaceEnvVar), "failed to load environment variable")
		os.Exit(78)
	}

	logger.Info("starting runner", "namespace", namespace, "pod", pod)

	config.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", "kube-system", "cronjob-controller"),
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error(err, "failed to create clientset")
		os.Exit(1)
	}

	_, err = clientSet.BatchV1().CronJobs(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get runner cronjob")
		os.Exit(1)
	}
}
