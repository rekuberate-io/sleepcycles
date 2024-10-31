package main

import (
	"context"
	"crypto/rand"
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
	"strings"
)

var logger *zap.Logger

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func main() {
	var kubeconfig *string
	home := homedir.HomeDir()

	if home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	purge := flag.Bool("purge", false, "(optional) false: remove all deployments , true: provision deployments")
	deployments := flag.Int("deployments", 10, "(optional) number of deployments")
	sleepcycle := flag.String("sleepcycle", "", "target sleepcycle")
	namespace := flag.String("namespace", apiv1.NamespaceDefault, "(optional) target kubernetes namespace")

	flag.Parse()

	if sleepcycle == nil || *sleepcycle == "" {
		flag.Usage()
		os.Exit(1)
	}

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
		err := deleteDeployments(
			deploymentsClient,
			*deployments,
			*sleepcycle,
		)
		if err != nil {
			gracefulExitCode = 1
			logger.Error(err.Error())
		}

		os.Exit(gracefulExitCode)
	}

	for i := 0; i < *deployments; i++ {
		deploymentSpec := getManifest(*sleepcycle)

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

	return nil
}

func deleteDeployments(deploymentsClient v1.DeploymentInterface, count int, sleepcycle string) error {
	logger.Info(fmt.Sprintf("attempting to delete %d deployments", count))

	listOptions := metav1.ListOptions{
		LabelSelector: "rekuberate.io/sleepcycle=" + sleepcycle,
	}
	deploymentsList, err := deploymentsClient.List(context.TODO(), listOptions)
	if err != nil {
		logger.Error("getting deployments list failed")
	}

	if len(deploymentsList.Items) < count {
		count = len(deploymentsList.Items)
	}

	for i := 0; i < count; i++ {
		deletePolicy := metav1.DeletePropagationForeground
		if err := deploymentsClient.Delete(context.TODO(), deploymentsList.Items[i].Name, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("deleted deployment: %s", deploymentsList.Items[i].Name))
	}

	return nil
}

func getManifest(sleepcycle string) *appsv1.Deployment {
	appName := fmt.Sprintf("whoami-%s", strings.ToLower(generateSecureRandomString(5)))

	deploymentSpec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
			Labels: map[string]string{
				"app":                      appName,
				"rekuberate.io/sleepcycle": sleepcycle,
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

func generateSecureRandomString(length int) string {
	result := make([]byte, length)
	_, err := rand.Read(result)
	if err != nil {
		os.Exit(1)
	}

	for i := range result {
		result[i] = letters[int(result[i])%len(letters)]
	}
	return string(result)
}
