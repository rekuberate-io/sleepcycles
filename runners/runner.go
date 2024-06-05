package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"strings"
	"time"
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

	eventRecorder record.EventRecorder
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
	}

	ns, ok := os.LookupEnv(namespaceEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, namespaceEnvVar), "failed to load environment variable")
	}

	cj, ok := os.LookupEnv(cronjobEnvVar)
	if !ok {
		logger.Error(fmt.Errorf(envVarErr, cronjobEnvVar), "failed to load environment variable")
	}

	logger.Info("starting runner", "namespace", ns, "cronjob", cj, "pod", pd)
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error(err, "failed to create clientset")
		//os.Exit(1)
	}
	clientSet = cs

	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)

	eventBroadcaster := record.NewBroadcaster()
	defer eventBroadcaster.Shutdown()

	eventBroadcaster.StartStructuredLogging(4)
	eventBroadcaster.StartRecordingToSink(&typedv1core.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})
	eventRecorder = eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: "rekuberate-io/sleepcycles-runner"})

	cronjob, err := clientSet.BatchV1().CronJobs(ns).Get(ctx, cj, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get runner cronjob")
		os.Exit(1)
	}

	opCode := getOpCode(cronjob.Name)
	target := cronjob.Labels["rekuberate.io/target"]
	kind := cronjob.Labels["rekuberate.io/target-kind"]

	replicas := int64(1)
	if kind != "CronJob" {
		replicas, err = strconv.ParseInt(cronjob.Annotations["rekuberate.io/replicas"], 10, 32)
		if err != nil {
			logger.Error(err, "failed to get rekuberate.io/replicas value")
		}
	}

	if err == nil {
		err := run(ns, cronjob, target, kind, replicas, isShutdownOp)
		if err != nil {
			recordEvent(cronjob, err.Error(), true)
			logger.Error(err, "runner failed", "target", target, "kind", kind)
		} else {
			action := "up"
			if opCode == 0 {
				action = "down"
			}
			recordEvent(cronjob, fmt.Sprintf("runner scaled %s %s", action, target), false)
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
		err := scaleStatefulSets(ctx, ns, cronjob, target, int32(targetReplicas))
		if err != nil {
			serr = errors.Wrap(err, smsg)
		}
	case "CronJob":
		if shutdown {
			targetReplicas = 0
		}
		err := scaleCronJob(ctx, ns, cronjob, target, int32(targetReplicas))
		if err != nil {
			serr = errors.Wrap(err, smsg)
		}
	case "HorizontalPodAutoscaler":
		if shutdown {
			targetReplicas = 1
		}
		err := scaleHorizontalPodAutoscalers(ctx, ns, cronjob, target, int32(targetReplicas))
		if err != nil {
			serr = errors.Wrap(err, smsg)
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

func scaleCronJob(ctx context.Context, namespace string, cronjob *batchv1.CronJob, target string, targetReplicas int32) error {
	cj, err := clientSet.BatchV1().CronJobs(namespace).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return err
	}

	suspend := targetReplicas <= 0
	if suspend != *cj.Spec.Suspend {
		cj.Spec.Suspend = &suspend
		_, err = clientSet.BatchV1().CronJobs(namespace).Update(ctx, cj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		action := "resumed"
		if suspend {
			action = "suspended"
		}

		logger.Info(fmt.Sprintf("cronjob %s", action), "namespace", namespace, "cronjob", target)
		return nil
	}

	logger.Info("cronjob already in desired state", "namespace", namespace, "cronjob", target, "suspended", suspend)
	return nil
}

func scaleStatefulSets(ctx context.Context, namespace string, cronjob *batchv1.CronJob, target string, targetReplicas int32) error {
	statefulSet, err := clientSet.AppsV1().StatefulSets(namespace).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := *statefulSet.Spec.Replicas
	err = syncReplicas(ctx, namespace, cronjob, currentReplicas, targetReplicas)
	if err != nil {
		return err
	}

	if currentReplicas != targetReplicas {
		statefulSet.Spec.Replicas = &targetReplicas
		_, err = clientSet.AppsV1().StatefulSets(namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		action := "down"
		if targetReplicas > 0 {
			action = "up"
		}

		logger.Info(fmt.Sprintf("scaled %s statefulset", action), "namespace", namespace, "statefulset", target, "replicas", targetReplicas)
		return nil
	}

	logger.Info("statefulset already in desired state", "namespace", namespace, "statefulset", target, "replicas", targetReplicas)

	return nil
}

func scaleHorizontalPodAutoscalers(ctx context.Context, namespace string, cronjob *batchv1.CronJob, target string, targetReplicas int32) error {
	hpa, err := clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(ctx, target, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currentReplicas := hpa.Spec.MaxReplicas
	err = syncReplicas(ctx, namespace, cronjob, currentReplicas, targetReplicas)
	if err != nil {
		return err
	}

	if currentReplicas != targetReplicas {
		hpa.Spec.MaxReplicas = targetReplicas
		_, err = clientSet.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(ctx, hpa, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		action := "down"
		if targetReplicas > 0 {
			action = "up"
		}

		logger.Info(fmt.Sprintf("scaled max replicas %s", action), "namespace", namespace, "hpa", target, "replicas", targetReplicas)
		return nil
	}

	logger.Info("horizontal pod autoscaler already in desired state", "namespace", namespace, "hpa", target, "replicas", targetReplicas)

	return nil
}

func recordEvent(cronjob *batchv1.CronJob, message string, isError bool) {
	eventType := corev1.EventTypeNormal
	reason := "SuccessfulSleepCycleScale"

	if isError {
		eventType = corev1.EventTypeWarning
		reason = "FailedSleepCycleScale"
	}

	eventRecorder.Event(cronjob, eventType, reason, strings.ToLower(message))
	time.Sleep(2 * time.Second)
}

type OpCode int

const (
	Terminate OpCode = iota - 1
	Shutdown
	Wakeup
)

func (o OpCode) String() string {
	suffix := "shutdown"
	switch o {
	case Terminate:
		suffix = "terminate"
	case Wakeup:
		suffix = "wakeup"
	case Shutdown:
		suffix = "shutdown"
	}

	return suffix
}

func getOpCode(cronJobName string) OpCode {
	if strings.HasSuffix(cronJobName, "terminate") {
		return Terminate
	}

	if strings.HasSuffix(cronJobName, "shutdown") {
		return Shutdown
	}

	if strings.HasSuffix(cronJobName, "wakeup") {
		return Wakeup
	}

	return Wakeup
}
