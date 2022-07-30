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

type DHCPLease struct {
	Subnet       string      `json:"subnet"`
	MAC          string      `json:"mac"`
	IP           string      `json:"ip,omitempty"`
	Gateway      string      `json:"gateway,omitempty"`
	HostName     string      `json:"hostname,omitempty"`
	DNS          []string    `json:"dns,omitempty"`
	Options      []Option    `json:"options,omitempty"`
	BootFileName string      `json:"bootFileName,omitempty"`
	ServerId     string      `json:"serverId"`
	LeaseTime    int         `json:"leaseTime,omitempty"`
	LastUpdate   metav1.Time `json:"lastUpdate,omitempty"`
	AckSent      bool        `json:"ackSent"`
}

// DHCPLeasesSpec defines the desired state of DHCPLeases
type DHCPLeasesSpec struct {
	NumLeases int                  `json:"numLeases"`
	Leases    map[string]DHCPLease `json:"leases"`
}

// DHCPLeasesStatus defines the observed state of DHCPLease
type DHCPLeasesStatus struct {
	ErrorMessage string `json:"errorMessage"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="numLeases",type="integer",JSONPath=".spec.numLeases",description="Leases count",priority=0

// DHCPLeases is the Schema for the dhcpleases API
type DHCPLeases struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DHCPLeasesSpec   `json:"spec,omitempty"`
	Status DHCPLeasesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DHCPLeasesList contains a list of DHCPLeases
type DHCPLeasesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DHCPLeases `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DHCPLeases{}, &DHCPLeasesList{})
}

func (s *DHCPLease) ToLease() *dhcp.Lease {
	lease := dhcp.Lease{
		Subnet:       dhcp.SubnetAddrPrefix(s.Subnet),
		MAC:          s.MAC,
		IP:           net.ParseIP(s.IP),
		Gateway:      net.ParseIP(s.Gateway),
		BootFileName: s.BootFileName,
		DNS:          s.DNS,
		LeaseTime:    s.LeaseTime,
		HostName:     s.HostName,
		ServerId:     net.ParseIP(s.ServerId),
		LastUpdate:   s.LastUpdate.Time,
		AckSent:      s.AckSent,
		Options:      []dhcp.Option{},
	}
	for _, opt := range s.Options {
		lease.Options = append(lease.Options, dhcp.Option{
			ID:    opt.ID,
			Type:  opt.Type,
			Value: opt.Value,
		})
	}
	return &lease
}

func NewDHCPLeaseFromLease(lease *dhcp.Lease) DHCPLease {
	l := DHCPLease{
		Subnet:       string(lease.Subnet),
		MAC:          lease.MAC,
		IP:           lease.IP.String(),
		Gateway:      lease.Gateway.String(),
		HostName:     lease.HostName,
		DNS:          lease.DNS,
		BootFileName: lease.BootFileName,
		ServerId:     lease.ServerId.String(),
		LeaseTime:    lease.LeaseTime,
		LastUpdate:   metav1.Time{Time: lease.LastUpdate},
		Options:      []Option{},
	}
	for _, opt := range lease.Options {
		l.Options = append(l.Options, Option{
			ID:    opt.ID,
			Type:  opt.Type,
			Value: opt.Value,
		})
	}
	return l
}
