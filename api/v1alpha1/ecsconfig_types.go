/*
Copyright 2022 br4sk4.

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

// ECSConfigSpec defines the desired state of ECSConfig
type ECSConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Cluster        string   `json:"cluster,omitempty"`
	VpcId          string   `json:"vpcId,omitempty"`
	Subnets        []string `json:"subnets,omitempty"`
	SecurityGroups []string `json:"securityGroups,omitempty"`
	RegistryUrl    string   `json:"registryUrl,omitempty"`
	TaskRoleArn    string   `json:"taskRoleArn,omitempty"`
}

// ECSConfigStatus defines the observed state of ECSConfig
type ECSConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ECSConfig is the Schema for the ecsconfigs API
type ECSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ECSConfigSpec   `json:"spec,omitempty"`
	Status ECSConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ECSConfigList contains a list of ECSConfig
type ECSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ECSConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ECSConfig{}, &ECSConfigList{})
}
