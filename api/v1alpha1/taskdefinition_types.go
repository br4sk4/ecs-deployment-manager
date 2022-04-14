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
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ContainerDefinition struct {
	RegistryUrl   string `json:"registryUrl,omitempty"`
	Image         string `json:"image,omitempty"`
	ContainerPort int    `json:"containerPort,omitempty"`
	HostPort      int    `json:"hostPort,omitempty"`
}

// TaskDefinitionSpec defines the desired state of TaskDefinition
type TaskDefinitionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Cpu                 int                      `json:"cpu,omitempty"`
	Memory              int                      `json:"memory,omitempty"`
	Compatibilities     []ecsTypes.Compatibility `json:"compatibilities,omitempty"`
	NetworkMode         ecsTypes.NetworkMode     `json:"networkMode,omitempty"`
	TaskRoleArn         string                   `json:"taskRoleArn,omitempty"`
	ContainerDefinition ContainerDefinition      `json:"containerDefinition,omitempty"`
}

// TaskDefinitionStatus defines the observed state of TaskDefinition
type TaskDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Synced            bool   `json:"synced,omitempty"`
	TaskDefinitionArn string `json:"taskDefinitionArn,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TaskDefinition is the Schema for the taskdefinitions API
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.synced`
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.taskDefinitionArn`
type TaskDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskDefinitionSpec   `json:"spec,omitempty"`
	Status TaskDefinitionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TaskDefinitionList contains a list of TaskDefinition
type TaskDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TaskDefinition{}, &TaskDefinitionList{})
}
