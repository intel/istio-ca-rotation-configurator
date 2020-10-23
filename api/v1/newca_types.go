/*


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

// +kubebuilder:validation:Enum=Complete;In progress;Failure
type RotationState string

const (
	// CompleteRotation meanst that no CA rotation is in progress.
	CompleteRotation RotationState = "Complete"

	// InProgressRotation means that the rotation is happening right now.
	InProgressRotation RotationState = "In progress"

	// FailedRotation means that the rotation has failed.
	FailedRotation RotationState = "Failure"
)

// NewCASpec defines the desired state of NewCA
type NewCASpec struct {
	// Name of the secret.
	Secret string `json:"secret,omitempty"`

	// Namespace of the secret.
	Namespace string `json:"namespace,omitempty"`
}

// NewCAStatus defines the observed state of NewCA
type NewCAStatus struct {
	// Status tells if the cluster has succeeded in rotating the Istio CA. Possible
	// values: "Complete", "In progress", "Failure"
	Status RotationState `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NewCA is the Schema for the newcas API
type NewCA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NewCASpec   `json:"spec,omitempty"`
	Status NewCAStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NewCAList contains a list of NewCA
type NewCAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NewCA `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NewCA{}, &NewCAList{})
}
