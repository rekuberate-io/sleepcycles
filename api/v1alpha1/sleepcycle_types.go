/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SleepCycleSpec defines the desired state of SleepCycle
type SleepCycleSpec struct {
	// +kubebuilder:validation:Pattern:=`(^((\*\/)?([0-5]?[0-9])((\,|\-|\/)([0-5]?[0-9]))*|\*)\s+((\*\/)?((2[0-3]|1[0-9]|[0-9]|00))((\,|\-|\/)(2[0-3]|1[0-9]|[0-9]|00))*|\*)\s+((\*\/)?([1-9]|[12][0-9]|3[01])((\,|\-|\/)([1-9]|[12][0-9]|3[01]))*|\*)\s+((\*\/)?([1-9]|1[0-2])((\,|\-|\/)([1-9]|1[0-2]))*|\*|(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|des))\s+((\*\/)?[0-6]((\,|\-|\/)[0-6])*|\*|00|(sun|mon|tue|wed|thu|fri|sat))\s*$)|@(annually|yearly|monthly|weekly|daily|hourly|reboot)`
	// +kubebuilder:validation:Type=string
	Shutdown string `json:"shutdown"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="UTC"
	ShutdownTimeZone *string `json:"shutdownTimeZone,omitempty"`

	// +kubebuilder:validation:Pattern:=`(^((\*\/)?([0-5]?[0-9])((\,|\-|\/)([0-5]?[0-9]))*|\*)\s+((\*\/)?((2[0-3]|1[0-9]|[0-9]|00))((\,|\-|\/)(2[0-3]|1[0-9]|[0-9]|00))*|\*)\s+((\*\/)?([1-9]|[12][0-9]|3[01])((\,|\-|\/)([1-9]|[12][0-9]|3[01]))*|\*)\s+((\*\/)?([1-9]|1[0-2])((\,|\-|\/)([1-9]|1[0-2]))*|\*|(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|des))\s+((\*\/)?[0-6]((\,|\-|\/)[0-6])*|\*|00|(sun|mon|tue|wed|thu|fri|sat))\s*$)|@(annually|yearly|monthly|weekly|daily|hourly|reboot)`
	// +kubebuilder:validation:Type=string
	WakeUp *string `json:"wakeup,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="UTC"
	WakeupTimeZone *string `json:"wakeupTimeZone,omitempty"`

	// +kubebuilder:validation:default:=true
	// +kubebuilder:validation:Type=boolean
	Enabled bool `json:"enabled"`

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Type=integer
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:validation:ExclusiveMinimum=false
	// +kubebuilder:validation:ExclusiveMaximum=false
	SuccessfulJobsHistoryLimit int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Type=integer
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:validation:ExclusiveMinimum=false
	// +kubebuilder:validation:ExclusiveMaximum=false
	FailedJobsHistoryLimit int32 `json:"failedJobsHistoryLimit,omitempty"`

	Runner RunnerConfig `json:"runner,omitempty"`
}

type Metadata struct {
	// Additionnal annotation to merge to the resource associated
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/#syntax-and-character-set
	Annotations map[string]string `json:"annotations,omitempty"`
	// Additionnal labels to merge to the resource associated
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
	Labels map[string]string `json:"labels,omitempty"`
}

// RunnerConfig defines the configuration of runner image
type RunnerConfig struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="akyriako78/rekuberate-io-sleepcycles-runners"
	Image string `json:"runnerImage,omitempty"`

	// RunAsUser define the id of the user to run in the image
	// +kubebuilder:validation:Minimum=1
	RunAsUser *int64 `json:"runAsUser,omitempty"`

	// imagePullPolicy define the pull policy for docker image
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// resourceRequirements works exactly like Container resources, the user can specify the limit and the requests
	// through this property
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +kubebuilder:validation:Optional
	ResourcesRequirements *corev1.ResourceRequirements `json:"resourcesRequirements,omitempty"`
	// imagePullSecrets specifies the secret to use when using private registry
	// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#localobjectreference-v1-core
	// +kubebuilder:validation:Optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// nodeSelector can be specified, which set the pod to fit on a node
	// https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// tolerations can be specified, which set the pod's tolerations
	// https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/#concepts
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// podMetadata allows to add additionnal metadata to the node pods
	PodMetadata Metadata `json:"podMetadata,omitempty"`

	// priorityClassName can be used to set the priority class applied to the node
	// +optional
	PriorityClassName *string `json:"priorityClassName,omitempty"`
}

// GetResources returns the sleepcycle runner Kubernetes resource.
func (r *RunnerConfig) GetResources() *v1.ResourceRequirements {
	if r.ResourcesRequirements != nil {
		return r.ResourcesRequirements
	}
	return &v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    resource.MustParse("10m"),
			"memory": resource.MustParse("128mb"),
		},
		Requests: v1.ResourceList{
			"cpu":    resource.MustParse("10m"),
			"memory": resource.MustParse("128mb"),
		},
	}
}

// GetTolerations returns the tolerations for the given node.
func (r *RunnerConfig) GetTolerations() []corev1.Toleration {
	return r.Tolerations
}

// GetNodeSelector returns the node selector for the given node.
func (r *RunnerConfig) GetNodeSelector() map[string]string {
	return r.NodeSelector
}

// GetImagePullSecrets returns the list of Secrets needed to pull Containers images from private repositories.
func (r *RunnerConfig) GetImagePullSecrets() []corev1.LocalObjectReference {
	return r.ImagePullSecrets
}

// GetImagePullPolicy returns the image pull policy to pull containers images.
func (r *RunnerConfig) GetImagePullPolicy() corev1.PullPolicy {
	return r.ImagePullPolicy
}

func (r *RunnerConfig) GetPodAnnotations() map[string]string {
	return r.PodMetadata.Annotations
}

// GetNodeLabels returns additional labels configured to be applied to each nifi node.
func (r *RunnerConfig) GetPodLabels() map[string]string {
	return r.PodMetadata.Labels
}

// GetPriorityClass returns the name of the priority class to use for the given node.
func (r *RunnerConfig) GetPriorityClass() string {
	if r.PriorityClassName != nil {
		return *r.PriorityClassName
	}
	return ""
}

// SleepCycleStatus defines the observed state of SleepCycle
type SleepCycleStatus struct {
	Enabled bool   `json:"enabled,omitempty"`
	State   string `json:"state,omitempty"`
	Targets string `json:"targets,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SleepCycle is the Schema for the sleepcycles API
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Targets",type=string,JSONPath=`.status.targets`
// +kubebuilder:printcolumn:name="Shutdown Schedule",type=string,JSONPath=`.spec.shutdown`
// +kubebuilder:printcolumn:name="Shutdown Timezone",type=string,JSONPath=`.spec.shutdownTimeZone`
// +kubebuilder:printcolumn:name="Wakeup Schedule",type=string,JSONPath=`.spec.wakeup`
// +kubebuilder:printcolumn:name="Wakeup Timezone",type=string,JSONPath=`.spec.wakeupTimeZone`
type SleepCycle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SleepCycleSpec   `json:"spec,omitempty"`
	Status SleepCycleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SleepCycleList contains a list of SleepCycle
type SleepCycleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SleepCycle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SleepCycle{}, &SleepCycleList{})
}
