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
	"strconv"
	"strings"
)

var (
	logger    logr.Logger
	config    *rest.Config
	clientSet *kubernetes.Clientset
	ctx       context.Context

	envVarErr       = "environment variable %s not found"
	podEnvVar       = "MY_POD_NAME"
	namespaceEnvVar = "MY_POD_NAMESPACE"
	cronjobEnvVar   = "MY_CRONJOB_NAME"
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
	pd, ok := os.LookupEnv(podEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, podEnvVar), "failed to load environment variable")
		os.Exit(78)
	}

	ns, ok := os.LookupEnv(namespaceEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, namespaceEnvVar), "failed to load environment variable")
		os.Exit(78)
	}

	cj, ok := os.LookupEnv(cronjobEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, cronjobEnvVar), "failed to load environment variable")
		os.Exit(78)
	}

	logger.Info("starting runner", "namespace", ns, "cronjob", cj, "pod", pd)
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error(err, "failed to create clientset")
		os.Exit(1)
	}
	clientSet = cs

	cronjob, err := clientSet.BatchV1().CronJobs(ns).Get(ctx, cj, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get runner cronjob")
		os.Exit(1)
	}

	isShutdownOp := true
	if !strings.HasSuffix(cronjob.Name, "shutdown") {
		isShutdownOp = false
	}

	replicas, err := strconv.ParseInt(cronjob.Annotations["rekuberate.io/replicas"], 10, 32)
	if err != nil {
		logger.Error(err, "failed to get rekuberate.io/replicas value")
		os.Exit(1)
	}

	target := cronjob.Labels["rekuberate.io/target"]
	kind := cronjob.Labels["rekuberate.io/target-kind"]
	switch kind {
	case "Deployment":
		if isShutdownOp {
			replicas = 0
		}
		err := scaleDeployment(ctx, ns, target, int32(replicas))
		if err != nil {
			logger.Error(err, "scaling failed", "kind", kind, "target", target)
			os.Exit(1)
		}
	case "StatefulSet":
		if isShutdownOp {
			replicas = 0
		}
	case "CronJob":
		if isShutdownOp {
			replicas = 0
		}
	case "HorizontalPodAutoscaler":
		if isShutdownOp {
			replicas = 1
		}
	default:
		logger.Error(err, "not supported kind", "kind", kind)
		os.Exit(1)
	}
}

func scaleDeployment(ctx context.Context, namespace string, target string, replicas int32) error {
	deployment, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	_, err = clientSet.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	logger.Info("scaled deployment", "namespace", namespace, "deployment", target, "replicas", replicas)
	return nil
}
