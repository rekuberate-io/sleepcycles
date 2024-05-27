package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	batchv1 "k8s.io/api/batch/v1"
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
		//os.Exit(78)
	}

	ns, ok := os.LookupEnv(namespaceEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, namespaceEnvVar), "failed to load environment variable")
		//os.Exit(78)
	}

	cj, ok := os.LookupEnv(cronjobEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, cronjobEnvVar), "failed to load environment variable")
		//os.Exit(78)
	}

	logger.Info("starting runner", "namespace", ns, "cronjob", cj, "pod", pd)
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error(err, "failed to create clientset")
		//os.Exit(1)
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
	}

	target := cronjob.Labels["rekuberate.io/target"]
	kind := cronjob.Labels["rekuberate.io/target-kind"]

	if err == nil {
		err := run(ns, cronjob, target, kind, replicas, isShutdownOp)
		if err != nil {
			logger.Error(err, "runner failed", "target", target, "kind", kind)
		}
	}
}

func run(ns string, cronjob *batchv1.CronJob, target string, kind string, targetReplicas int64, shutdown bool) error {
	smsg := "scaling failed"
	var serr error

	switch kind {
	case "Deployment":
		if shutdown {
			targetReplicas = 0
		}
		err := scaleDeployment(ctx, ns, cronjob, target, int32(targetReplicas))
		if err != nil {
			serr = errors.Wrap(err, smsg)
		}
	case "StatefulSet":
		if shutdown {
			targetReplicas = 0
		}
	case "CronJob":
		if shutdown {
			targetReplicas = 0
		}
	case "HorizontalPodAutoscaler":
		if shutdown {
			targetReplicas = 1
		}
	default:
		err := fmt.Errorf("not supported kind: %s", kind)
		serr = errors.Wrap(err, smsg)
	}

	return serr
}

func syncReplicas(ctx context.Context, namespace string, cronjob *batchv1.CronJob, currentReplicas int32, targetReplicas int32) error {
	if currentReplicas != targetReplicas && currentReplicas > 0 {
		if targetReplicas != 0 {
			targetReplicas = currentReplicas
		}

		cronjob.Annotations["rekuberate.io/replicas"] = fmt.Sprint(currentReplicas)
		_, err := clientSet.BatchV1().CronJobs(namespace).Update(ctx, cronjob, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func scaleDeployment(ctx context.Context, namespace string, cronjob *batchv1.CronJob, target string, targetReplicas int32) error {
	deployment, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *deployment.Spec.Replicas
	err = syncReplicas(ctx, namespace, cronjob, currentReplicas, targetReplicas)
	if err != nil {
		return err
	}

	if currentReplicas != targetReplicas {
		deployment.Spec.Replicas = &targetReplicas
		_, err = clientSet.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		action := "down"
		if targetReplicas > 0 {
			action = "up"
		}

		logger.Info(fmt.Sprintf("scaled %s deployment", action), "namespace", namespace, "deployment", target, "replicas", targetReplicas)
		return nil
	}

	logger.Info("deployment already in desired state", "namespace", namespace, "deployment", target, "replicas", targetReplicas)

	return nil
}
