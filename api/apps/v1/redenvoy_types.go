/*
Copyright 2026 Abstract Prism.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodConfig defines the configuration for a managed pod, including its name and environment variables.
type PodConfig struct {
	Name string            `json:"name"`
	Envs map[string]string `json:"envs,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedEnvoySpec defines the desired state of RedEnvoy
type RedEnvoySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	ContainerImage string `json:"containerImage,omitempty"`
	// +optional
	TriggeredEventName string `json:"triggeredEventName,omitempty"`
	// +optional
	TriggeredDeleteEventName string `json:"triggeredDeleteEventName,omitempty"`
	// ContainerPort is the port that the container exposes. Defaults to 8080.
	// +optional
	ContainerPort *int32 `json:"containerPort,omitempty"`
	// IngressHost is the hostname for the Ingress resource. Required to create an Ingress.
	// +optional
	IngressHost string `json:"ingressHost,omitempty"`
}

// RedEnvoyStatus defines the observed state of RedEnvoy.
type RedEnvoyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the RedEnvoy resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ManagedPods is the list of pod configurations that are currently being managed by this operator.
	// +optional
	ManagedPods []PodConfig `json:"managedPods,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RedEnvoy is the Schema for the redenvoys API
type RedEnvoy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RedEnvoy
	// +required
	Spec RedEnvoySpec `json:"spec"`

	// status defines the observed state of RedEnvoy
	// +optional
	Status RedEnvoyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RedEnvoyList contains a list of RedEnvoy
type RedEnvoyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RedEnvoy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedEnvoy{}, &RedEnvoyList{})
}
