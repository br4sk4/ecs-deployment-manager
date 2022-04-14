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

type HealthCheck struct {
	Enabled            bool   `json:"enabled,omitempty"`
	Path               string `json:"path,omitempty"`
	Protocol           string `json:"protocol,omitempty"`
	Interval           int32  `json:"interval,omitempty"`
	Timeout            int32  `json:"timeout,omitempty"`
	HealthyThreshold   int32  `json:"healthyThreshold,omitempty"`
	UnhealthyThreshold int32  `json:"unhealthyThreshold,omitempty"`
}

type LoadBalancer struct {
	Name        string `json:"name,omitempty"`
	PathPattern string `json:"pathPattern,omitempty"`
}

// TargetGroupSpec defines the desired state of TargetGroup
type TargetGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of TargetGroup. Edit targetgroup_types.go to remove/update
	//Foo string `json:"foo,omitempty"`

	VpcId           string       `json:"vpcId,omitempty"`
	TargetType      string       `json:"targetType,omitempty"`
	Port            int32        `json:"port,omitempty"`
	Protocol        string       `json:"protocol,omitempty"`
	ProtocolVersion string       `json:"protocolVersion,omitempty"`
	LoadBalancer    LoadBalancer `json:"loadBalancer,omitempty"`
	HealthCheck     HealthCheck  `json:"healthCheck,omitempty"`
}

// TargetGroupStatus defines the observed state of TargetGroup
type TargetGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Synced          bool   `json:"synced,omitempty"`
	LoadBalancerArn string `json:"loadBalancerArn,omitempty"`
	ListenerArn     string `json:"listenerArn,omitempty"`
	ListenerRuleArn string `json:"listenerRuleArn,omitempty"`
	TargetGroupArn  string `json:"targetGroupArn,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TargetGroup is the Schema for the targetgroups API
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.synced`
// +kubebuilder:printcolumn:name="ARN",type=string,JSONPath=`.status.targetGroupArn`
type TargetGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetGroupSpec   `json:"spec,omitempty"`
	Status TargetGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TargetGroupList contains a list of TargetGroup
type TargetGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TargetGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TargetGroup{}, &TargetGroupList{})
}
