/*
Copyright 2022 br4sk4.

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
	"k8s.io/apimachinery/pkg/runtime"
	ecsdeploymentmanagerv1alpha1 "naffets.eu/ecs-deployment-manager/api/v1alpha1"
	"naffets.eu/ecs-deployment-manager/reconcilers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const serviceFinalizerName = "naffets.eu/ServiceFinalizer"

//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=ecsconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Service object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var service ecsdeploymentmanagerv1alpha1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err == nil {
		reconcilerClient := reconcilers.NewServiceReconcilerClient(r.Client, &service)
		return reconcilerClient.Reconcile(ctx), nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecsdeploymentmanagerv1alpha1.Service{}).
		Complete(r)
}
