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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/log"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/bmcgo/k8s-dhcp/dhcp"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dhcpv1alpha1 "github.com/bmcgo/k8s-dhcp/api/v1alpha1"
	"github.com/bmcgo/k8s-dhcp/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(dhcpv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()
	logger := &Logger{rlog: log.FromContext(ctx)}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	knownObjectsStorage := controllers.NewObjectsCache()

	serverReconciler := controllers.NewDHCPServerReconciler(mgr.GetClient(), mgr.GetScheme(), knownObjectsStorage, logger)
	if err = serverReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DHCPServer")
		os.Exit(1)
	}

	subnetReconciler := controllers.NewDHCPSubnetReconciler(mgr.GetClient(), mgr.GetScheme(), knownObjectsStorage)
	if err = subnetReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DHCPSubnet")
		os.Exit(1)
	}

	hostReconciler := controllers.NewDHCPHostReconciler(mgr.GetClient(), mgr.GetScheme(), knownObjectsStorage)
	if err = hostReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DHCPLease")
		os.Exit(1)
	}
	if err = (&controllers.DHCPHostReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DHCPHost")
		os.Exit(1)
	}

	leasesReconciler := controllers.NewDHCPLeaseReconciler(mgr.GetClient(), mgr.GetScheme(), knownObjectsStorage, logger)
	if err = leasesReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DHCPLease")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	dhcpServer, err := dhcp.NewServer(dhcp.ServerConfig{
		Logger:             logger,
		CallbackSaveLeases: leasesReconciler.CallbackSaveLeases,
		Context:            ctx,
	})
	if err != nil {
		setupLog.Error(err, "failed to create server")
		os.Exit(2)
	}
	defer dhcpServer.Close()
	serverReconciler.DHCPServer = dhcpServer
	subnetReconciler.DHCPServer = dhcpServer
	hostReconciler.DHCPServer = dhcpServer
	leasesReconciler.DHCPServer = dhcpServer

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type Logger struct {
	rlog logr.Logger
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.rlog.V(0).Info(fmt.Sprintf(format, args...))
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.rlog.V(1).Info(fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(err error, format string, args ...interface{}) {
	l.rlog.Error(err, fmt.Sprintf(format, args...))
}

func (l *Logger) WithName(name string) dhcp.RLogger {
	return &Logger{rlog: l.rlog.WithName(name)}
}
