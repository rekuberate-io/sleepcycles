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

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="akyriako78/rekuberate-io-sleepcycles-runners"
	RunnerImage string `json:"runnerImage,omitempty"`
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
