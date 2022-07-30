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

// DHCPHostReconciler reconciles a DHCPHost object
type DHCPHostReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	DHCPServer *dhcp.Server

	hostsCache   map[string]dhcp.Host
	knownObjects *ObjectsCache
}

func NewDHCPHostReconciler(c client.Client, scheme *runtime.Scheme, knownObjects *ObjectsCache) *DHCPHostReconciler {
	return &DHCPHostReconciler{
		Client:       c,
		Scheme:       scheme,
		knownObjects: knownObjects,
	}
}

//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcphosts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcphosts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcphosts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DHCPHost object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DHCPHostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	key := req.Namespace + "/" + req.Name
	l.Info("reconcile", "host", req)
	host := dhcpv1alpha1.DHCPHost{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, &host)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("host deleted")
			sn, ok := r.hostsCache[key]
			if !ok {
				return ctrl.Result{Requeue: false}, fmt.Errorf("unknown host deleted %s", key)
			}
			err = r.DHCPServer.DeleteHost(sn)
			return ctrl.Result{Requeue: false}, err
		}
		l.Error(err, "Failed to load DHCPHost")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, err
	}

	saved := r.knownObjects.AddHostIfNotKnown(host)
	if !saved {
		err = r.DHCPServer.AddHost(host.ToDHCPHost())
		if err != nil {
			l.Error(err, "failed to add host")
		}
	} else {
		l.Info("cached host for not yet known subnet", "hostname", host.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DHCPHostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dhcpv1alpha1.DHCPHost{}).
		Complete(r)
}
