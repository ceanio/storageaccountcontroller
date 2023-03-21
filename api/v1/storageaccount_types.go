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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StorageAccountSpec defines the desired state of StorageAccount
type StorageAccountSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Properties StorageAccountProperties `json:"properties,omitempty"`
}

type StorageAccountProperties struct {
	// +kubebuilder:validation:Enum=Cool;Hot;Premium
	// +optional
	AccessTier string `json:"accessTier,omitempty"`
}

// StorageAccountStatus defines the observed state of StorageAccount
type StorageAccountStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Keys []StorageAccountKey `json:"keys"`
}

// +kubebuilder:validation:Enum=Full;Read
type StorageAccountPermission string

type StorageAccountKey struct {
	KeyName      string                   `json:"keyName"`
	Permissions  StorageAccountPermission `json:"permissions"`
	Value        string                   `json:"value"`
	CreationTime metav1.Time              `json:"creationTime"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// StorageAccount is the Schema for the storageaccounts API
type StorageAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageAccountSpec   `json:"spec,omitempty"`
	Status StorageAccountStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StorageAccountList contains a list of StorageAccount
type StorageAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageAccount{}, &StorageAccountList{})
}
