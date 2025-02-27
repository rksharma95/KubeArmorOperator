/*
Copyright 2024.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ImageSpec defines the image specifications
type ImageSpec struct {
	// +kubebuilder:validation:optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +kubebuilder:default:=Always
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`
}

func (i *ImageSpec) IsEmpty() bool {
	return *i == ImageSpec{}
}

type Tls struct {
	// +kubebuilder:validation:optional
	// +kubebuilder:default:=false
	Enable bool `json:"enable,omitempty"`
	// +kubebuilder:validation:optional
	RelayExtraDnsNames []string `json:"extraDnsNames,omitempty"`
	// +kubebuilder:validation:optional
	RelayExtraIpAddresses []string `json:"extraIpAddresses,omitempty"`
}

// KubeArmorConfigSpec defines the desired state of KubeArmorConfig
type KubeArmorConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:optional
	DefaultFilePosture PostureType `json:"defaultFilePosture,omitempty"`
	// +kubebuilder:validation:optional
	DefaultCapabilitiesPosture PostureType `json:"defaultCapabilitiesPosture,omitempty"`
	// +kubebuilder:validation:optional
	DefaultNetworkPosture PostureType `json:"defaultNetworkPosture,omitempty"`
	// +kubebuilder:validation:optional
	DefaultVisibility string `json:"defaultVisibility,omitempty"`
	// +kubebuilder:validation:optional
	KubeArmorImage ImageSpec `json:"kubearmorImage,omitempty"`
	// +kubebuilder:validation:optional
	KubeArmorInitImage ImageSpec `json:"kubearmorInitImage,omitempty"`
	// +kubebuilder:validation:optional
	KubeArmorRelayImage ImageSpec `json:"kubearmorRelayImage,omitempty"`
	// +kubebuilder:validation:optional
	KubeArmorControllerImage ImageSpec `json:"kubearmorControllerImage,omitempty"`
	// +kubebuilder:validation:optional
	KubeRbacProxyImage ImageSpec `json:"kubeRbacProxyImage,omitempty"`
	// +kubebuilder:validation:optional
	Tls Tls `json:"tls,omitempty"`
	// +kubebuilder:validation:optional
	EnableStdOutLogs bool `json:"enableStdOutLogs,omitempty"`
	// +kubebuilder:validation:optional
	EnableStdOutAlerts bool `json:"enableStdOutAlerts,omitempty"`
	// +kubebuilder:validation:optional
	EnableStdOutMsgs bool `json:"enableStdOutMsgs,omitempty"`
	// +kubebuilder:validation:Optional
	SeccompEnabled bool `json:"seccompEnabled,omitempty"`
	// +kubebuilder:validation:Optional
	AlertThrottling bool `json:"alertThrottling,omitempty"`
	// +kubebuilder:validation:Optional
	MaxAlertPerSec int `json:"maxAlertPerSec,omitempty"`
	// +kubebuilder:validation:Optional
	ThrottleSec int `json:"throttleSec,omitempty"`
}

// KubeArmorConfigStatus defines the observed state of KubeArmorConfig
type KubeArmorConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +kubebuilder:validation:optional
	Phase string `json:"phase,omitempty"`
	// +kubebuilder:validation:optional
	Message string `json:"message,omitempty"`
}

// KubeArmorConfig is the Schema for the kubearmorconfigs API
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
type KubeArmorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeArmorConfigSpec   `json:"spec,omitempty"`
	Status KubeArmorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeArmorConfigList contains a list of KubeArmorConfig
type KubeArmorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeArmorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeArmorConfig{}, &KubeArmorConfigList{})
}
