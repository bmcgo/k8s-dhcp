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
	"net"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DHCPHostSpec defines the desired state of DHCPHost
type DHCPHostSpec struct {
	Subnet         string   `json:"subnet"`
	MAC            string   `json:"mac"`
	IP             string   `json:"ip,omitempty"`
	Gateway        string   `json:"gateway,omitempty"`
	HostName       string   `json:"hostname,omitempty"`
	DNS            []string `json:"dns,omitempty"`
	Options        []Option `json:"options,omitempty"`
	ServerHostName string   `json:"serverHostName,omitempty"`
	BootFileName   string   `json:"bootFileName,omitempty"`
	LeaseTime      int      `json:"leaseTime,omitempty"`
}

// DHCPHostStatus defines the observed state of DHCPHost
type DHCPHostStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="mac",type="string",JSONPath=".spec.mac",description="MAC",priority=0
//+kubebuilder:printcolumn:name="ip",type="string",JSONPath=".spec.ip",description="IP",priority=0
//+kubebuilder:printcolumn:name="hostname",type="string",JSONPath=".spec.hostname",description="IP",priority=0

// DHCPHost is the Schema for the dhcphosts API
type DHCPHost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DHCPHostSpec   `json:"spec,omitempty"`
	Status DHCPHostStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DHCPHostList contains a list of DHCPHost
type DHCPHostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DHCPHost `json:"items"`
}

func (s *DHCPHost) ToDHCPHost() dhcp.Host {
	host := dhcp.Host{
		MAC:            s.Spec.MAC,
		IP:             net.ParseIP(s.Spec.IP),
		Gateway:        net.ParseIP(s.Spec.Gateway),
		ServerHostName: s.Spec.ServerHostName,
		BootFileName:   s.Spec.BootFileName,
		LeaseTime:      s.Spec.LeaseTime,
		HostName:       s.Spec.HostName,
		Options:        []dhcp.Option{},
		DNS:            s.Spec.DNS,
	}
	for _, opt := range s.Spec.Options {
		host.Options = append(host.Options, dhcp.Option{
			ID:    opt.ID,
			Type:  opt.Type,
			Value: opt.Value,
		})
	}
	return host
}

func init() {
	SchemeBuilder.Register(&DHCPHost{}, &DHCPHostList{})
}
