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
	"github.com/bmcgo/k8s-dhcp/dhcp"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dhcpv1alpha1 "github.com/bmcgo/k8s-dhcp/api/v1alpha1"
)

// DHCPServerReconciler reconciles a DHCPServer object
type DHCPServerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	DHCPServer *dhcp.Server

	cache *ObjectsCache
	log   dhcp.RLogger
}

func NewDHCPServerReconciler(c client.Client, scheme *runtime.Scheme, cache *ObjectsCache, log dhcp.RLogger) *DHCPServerReconciler {
	return &DHCPServerReconciler{
		Client: c,
		Scheme: scheme,
		cache:  cache,
		log:    log,
	}
}

//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DHCPServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DHCPServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.cache.ListensLock.Lock()
	defer r.cache.ListensLock.Unlock()

	l := log.FromContext(ctx)
	sv := &dhcpv1alpha1.DHCPServer{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, sv)

	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("deleted listen")
			err = r.DHCPServer.DeleteListen(req.Name)
			delete(r.cache.knownListens, req.Name)
			return ctrl.Result{Requeue: false}, err
		}
		l.Error(err, "failed to get Listen")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Second * 10,
		}, err
	}

	listenObj := r.cache.knownListens[req.Name]
	if listenObj != nil {
		l.Info("Listen is already known")
		//TODO: update listener
		err = r.DHCPServer.DeleteListen(req.Name)
		if err != nil {
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
		}
	}
	l.Info("New Listen", "obj", sv)
	err = r.DHCPServer.AddListen(sv.ToListen())
	if err != nil {
		l.Error(err, "Failed to add listen")
		sv.Status.ErrorMessage = err.Error()
	} else {
		r.cache.knownListens[req.Name] = sv
	}

	if len(r.cache.knownListens) == 0 {
		err = r.Initialize(ctx)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, err
}

func (r *DHCPServerReconciler) Initialize(ctx context.Context) error {
	r.log.Infof("Loading all subnets")
	subnetList := &dhcpv1alpha1.DHCPSubnetList{}
	err := r.Client.List(ctx, subnetList)
	if err != nil {
		return err
	}
	for _, sn := range subnetList.Items {
		err = r.DHCPServer.AddSubnet(sn.ToSubnet())
		if err != nil {
			return err
		}
	}

	r.log.Infof("Loading all leases")
	loaded := 0
	failed := 0
	leasesList := dhcpv1alpha1.DHCPLeasesList{}
	err = r.Client.List(ctx, &leasesList)
	if err != nil {
		return err
	}
	leasesMap := map[string]dhcpv1alpha1.DHCPLeases{}
	for _, leases := range leasesList.Items {
		leasesMap[leases.Name] = leases
	}
	oldest := ""
	for {
		for key, leases := range leasesMap {
			tmp := leasesMap[oldest].CreationTimestamp
			if oldest == "" || leases.CreationTimestamp.Before(&tmp) {
				oldest = key
			}
		}
		leases := leasesMap[oldest].Spec.Leases
		for _, lease := range leases {
			err = r.DHCPServer.AddLease(lease.ToLease())
			if err != nil {
				r.log.Errorf(err, "failed to add lease")
				failed++
			} else {
				loaded++
			}
		}
		delete(leasesMap, oldest)
		if len(leasesMap) == 0 {
			break
		}
		oldest = ""
	}
	r.log.Infof("Leases loaded: %d (failed %d)", loaded, failed)
	//TODO: load listeners
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DHCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dhcpv1alpha1.DHCPServer{}).
		Complete(r)
}
