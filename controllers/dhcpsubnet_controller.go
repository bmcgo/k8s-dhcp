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

package controllers

import (
	"context"
	"fmt"
	"github.com/bmcgo/k8s-dhcp/dhcp"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dhcpv1alpha1 "github.com/bmcgo/k8s-dhcp/api/v1alpha1"
)

// DHCPSubnetReconciler reconciles a DHCPSubnet object
type DHCPSubnetReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	DHCPServer   *dhcp.Server
	SubnetCache  map[string]dhcp.SubnetAddrPrefix
	knownObjects *ObjectsCache
}

func NewDHCPSubnetReconciler(c client.Client, scheme *runtime.Scheme, storage *ObjectsCache) *DHCPSubnetReconciler {
	return &DHCPSubnetReconciler{
		Client:       c,
		Scheme:       scheme,
		SubnetCache:  map[string]dhcp.SubnetAddrPrefix{},
		knownObjects: storage,
	}
}

//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpsubnets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpsubnets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpsubnets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DHCPSubnet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DHCPSubnetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Reconcile subnet")
	subnet := dhcpv1alpha1.DHCPSubnet{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &subnet)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("subnet deleted")
			sn, ok := r.SubnetCache[req.Name]
			if !ok {
				return ctrl.Result{Requeue: false}, fmt.Errorf("unknown subnet deleted %s", req.Name)
			}
			err = r.DHCPServer.DeleteSubnet(sn)
			return ctrl.Result{Requeue: false}, err
		}
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, err
	}

	s := subnet.ToSubnet()
	if !r.knownObjects.AddSubnetIfNotKnown(s) {
		l.Info("Subnet already known")
		return ctrl.Result{}, nil
	}
	r.SubnetCache[req.Name] = s.Subnet
	err = r.DHCPServer.AddSubnet(s)
	if err == nil {
		for _, host := range r.knownObjects.PopUnknownHosts(s.Subnet) {
			err := r.DHCPServer.AddHost(host.ToDHCPHost())
			if err != nil {
				l.Error(err, "Error adding previously saved host")
			} else {
				l.Info("Added previously saved host %s", host.Name)
			}
		}
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *DHCPSubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dhcpv1alpha1.DHCPSubnet{}).
		Complete(r)
}
