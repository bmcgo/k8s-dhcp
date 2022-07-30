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
	dhcpv1alpha1 "github.com/bmcgo/k8s-dhcp/api/v1alpha1"
	"github.com/bmcgo/k8s-dhcp/dhcp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const leasesBatchMaxSize = 1024

// DHCPLeaseReconciler reconciles a DHCPLease object
type DHCPLeaseReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	DHCPServer *dhcp.Server

	cache *ObjectsCache
	log   dhcp.RLogger
}

func NewDHCPLeaseReconciler(c client.Client, scheme *runtime.Scheme, storage *ObjectsCache, log dhcp.RLogger) *DHCPLeaseReconciler {
	return &DHCPLeaseReconciler{
		Client: c,
		Scheme: scheme,
		log:    log,
		cache:  storage,
	}
}

//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpleases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpleases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dhcp.kaas.mirantis.com,resources=dhcpleases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DHCPLease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DHCPLeaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	leases := &dhcpv1alpha1.DHCPLeases{}
	err := r.Client.Get(ctx, req.NamespacedName, leases)
	if err != nil {
		if errors.IsNotFound(err) {
			leasesObj := r.cache.PopLeases(req.Name)
			if leasesObj == nil {
				r.log.Debugf("leases obj %s was garbage collected", req.Name)
				return ctrl.Result{Requeue: false}, nil
			}
			knownLeases := r.cache.GetAllKnownLeasesExcept(leasesObj.Name)
			for _, lease := range leasesObj.Spec.Leases {
				if ok, _ := knownLeases[lease.MAC]; ok != true {
					err = r.DHCPServer.DeleteLease(lease.ToLease())
					if err != nil {
						r.log.Errorf(err, "failed to delete lease")
					}
				}
			}
			return ctrl.Result{Requeue: false}, nil
		}
	}
	leasesFound := r.cache.AddLeasesIfNotKnown(leases)
	if leasesFound != nil {
		//TODO: handle lease update
		r.log.Infof("Updated Leases object")
	}
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DHCPLeaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dhcpv1alpha1.DHCPLeases{}).
		Complete(r)
}

func (r *DHCPLeaseReconciler) CallbackSaveLeases(responses []dhcp.Response) error {
	r.cache.offersSavingLock.Lock()
	defer r.cache.offersSavingLock.Unlock()
	ctx := context.TODO()

	leases := dhcpv1alpha1.DHCPLeases{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "leases-",
			Namespace:    "default",
		},
		Spec: dhcpv1alpha1.DHCPLeasesSpec{
			Leases: map[string]dhcpv1alpha1.DHCPLease{},
		},
	}

	var batchesToDelete []string

	for _, b := range r.cache.knownLeasesBatch { //TODO: must be sorted
		if len(b.Spec.Leases)+len(leases.Spec.Leases)+len(responses) < leasesBatchMaxSize {
			for _, l := range b.Spec.Leases {
				leases.Spec.Leases[l.MAC] = l
			}
			batchesToDelete = append(batchesToDelete, b.Name)
		}
	}

	for _, resp := range responses {
		if resp.Response.MessageType() == dhcpv4.MessageTypeOffer && r.cache.HasLease(resp.Lease.MAC) {
			continue
		}
		leases.Spec.Leases[resp.Lease.MAC] = dhcpv1alpha1.NewDHCPLeaseFromLease(resp.Lease)
	}
	if len(leases.Spec.Leases) == 0 {
		return nil
	}
	leases.Spec.NumLeases = len(leases.Spec.Leases)
	err := r.Client.Create(ctx, &leases)
	if err != nil {
		return err
	}
	for _, name := range batchesToDelete {
		err := r.Client.Delete(ctx, &dhcpv1alpha1.DHCPLeases{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
		})
		if err != nil {
			r.log.Errorf(err, "failed to collect leases: %s", name)
		} else {
			delete(r.cache.knownLeasesBatch, name)
		}
	}
	return nil
}
