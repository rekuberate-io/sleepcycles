package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
)

var logger *zap.Logger

func main() {
	var kubeconfig *string
	home := homedir.HomeDir()

	if home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	purge := flag.Bool("purge", false, "(optional) remove all deployments false, provision deployments true")
	deployments := flag.Int("deployments", 10, "(optional) number of deployments")
	namespace := flag.String("namespace", apiv1.NamespaceDefault, "(optional) target kubernetes namespace")

	flag.Parse()

	logger, _ = zap.NewProduction()
	defer logger.Sync()

	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", *kubeconfig))
	if err != nil {
		logger.Fatal(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	deploymentsClient := clientset.AppsV1().Deployments(*namespace)
	gracefulExitCode := 0

	if *purge {
		for i := 0; i < *deployments; i++ {
			err := deleteDeployment(deploymentsClient, fmt.Sprintf("whoami-%d", i))
			if err != nil {
				gracefulExitCode = 1
				logger.Error(err.Error())
			}
		}

		os.Exit(gracefulExitCode)
	}

	for i := 0; i < *deployments; i++ {
		deploymentSpec := getManifest(i)

		err := createDeployment(deploymentsClient, deploymentSpec)
		if err != nil {
			gracefulExitCode = 1
			logger.Error(err.Error())
		}
	}

	os.Exit(gracefulExitCode)
}

func createDeployment(deploymentsClient v1.DeploymentInterface, deploymentSpec *appsv1.Deployment) error {
	logger.Info(fmt.Sprintf("creating deployment %q", deploymentSpec.GetObjectMeta().GetName()))

	_, err := deploymentsClient.Create(context.TODO(), deploymentSpec, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	//logger.Info(fmt.Sprintf("Created deployment %q.\n", deployment.GetObjectMeta().GetName()))

	return nil
}

func deleteDeployment(deploymentsClient v1.DeploymentInterface, deploymentName string) error {
	logger.Info(fmt.Sprintf("deleting deployment %q", deploymentName))

	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		return err
	}
	//logger.Info("Deleted deployment.")

	return nil
}

func getManifest(idx int) *appsv1.Deployment {
	appName := fmt.Sprintf("whoami-%v", idx)

	deploymentSpec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
			Labels: map[string]string{
				"app":                      appName,
				"rekuberate.io/sleepcycle": "sleepcycle-app-2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appName,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": appName,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "whoami",
							Image: "traefik/whoami",
						},
					},
				},
			},
		},
	}

	return deploymentSpec
}
