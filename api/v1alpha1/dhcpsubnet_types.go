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

// DHCPSubnetSpec defines the desired state of DHCPSubnet
type DHCPSubnetSpec struct {
	Subnet         string   `json:"subnet"`
	RangeFrom      string   `json:"rangeFrom"`
	RangeTo        string   `json:"rangeTo"`
	Gateway        string   `json:"gateway,omitempty"`
	DNS            []string `json:"dns,omitempty"`
	Options        []Option `json:"options,omitempty"`
	ServerHostName string   `json:"serverHostName,omitempty"`
	BootFileName   string   `json:"bootFileName,omitempty"`
	LeaseTime      int      `json:"leaseTime,omitempty"`

	Server metav1.OwnerReference `json:"server,omitempty"`
}

type Option struct {
	ID    uint8  `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// DHCPSubnetStatus defines the observed state of DHCPSubnet
type DHCPSubnetStatus struct {
	ErrorMessage string `json:"errorMessage"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="subnet",type="string",JSONPath=".spec.subnet",description="Subnet",priority=0
//+kubebuilder:printcolumn:name="from",type="string",JSONPath=".spec.rangeFrom",description="Range From",priority=0
//+kubebuilder:printcolumn:name="to",type="string",JSONPath=".spec.rangeTo",description="Range To",priority=0
//+kubebuilder:printcolumn:name="gateway",type="string",JSONPath=".spec.gateway",description="Default gateway",priority=0

// DHCPSubnet is the Schema for the dhcpsubnets API
type DHCPSubnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DHCPSubnetSpec   `json:"spec,omitempty"`
	Status DHCPSubnetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DHCPSubnetList contains a list of DHCPSubnet
type DHCPSubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DHCPSubnet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DHCPSubnet{}, &DHCPSubnetList{})
}

func (s *DHCPSubnet) ToSubnet() dhcp.Subnet {
	sn := dhcp.Subnet{
		Subnet:         dhcp.SubnetAddrPrefix(s.Spec.Subnet),
		RangeFrom:      s.Spec.RangeFrom,
		RangeTo:        s.Spec.RangeTo,
		Gateway:        s.Spec.Gateway,
		DNS:            s.Spec.DNS,
		Options:        []dhcp.Option{},
		LeaseTime:      s.Spec.LeaseTime,
		ServerHostName: s.Spec.ServerHostName,
		BootFileName:   s.Spec.BootFileName,
	}
	for _, opt := range s.Spec.Options {
		sn.Options = append(sn.Options, dhcp.Option{
			ID:    opt.ID,
			Type:  opt.Type,
			Value: opt.Value,
		})
	}
	return sn
}
