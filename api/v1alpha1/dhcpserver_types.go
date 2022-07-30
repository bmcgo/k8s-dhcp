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
	"github.com/bmcgo/k8s-dhcp/dhcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DHCPServerSpec defines the desired state of DHCPServer
type DHCPServerSpec struct {
	ListenInterface string `json:"listenInterface,omitempty"`
	ListenAddress   string `json:"listenAddress,omitempty"`
	ReuseAddr       bool   `json:"reuseAddr,omitempty"`
}

// DHCPServerStatus defines the observed state of DHCPServer
type DHCPServerStatus struct {
	ErrorMessage string      `json:"errorMessage"`
	LastUpdate   metav1.Time `json:"lastUpdate"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="interface",type="string",JSONPath=".spec.listenInterface",description="Listen interface",priority=0
//+kubebuilder:printcolumn:name="listen",type="string",JSONPath=".spec.listenAddress",description="Listen address",priority=0

// DHCPServer is the Schema for the dhcpservers API
type DHCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DHCPServerSpec   `json:"spec,omitempty"`
	Status DHCPServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DHCPServerList contains a list of DHCPServer
type DHCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DHCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DHCPServer{}, &DHCPServerList{})
}

func (s *DHCPServer) ToListen() dhcp.Listen {
	return dhcp.Listen{
		Name:      s.Name,
		Interface: s.Spec.ListenInterface,
		Addr:      s.Spec.ListenAddress,
	}
}
