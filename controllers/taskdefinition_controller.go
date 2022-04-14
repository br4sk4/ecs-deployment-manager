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

// TaskDefinitionReconciler reconciles a TaskDefinition object
type TaskDefinitionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const taskDedfinitionFinalizerName = "naffets.eu/TaskDefinitionFinalizer"

//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=taskdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=taskdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ecs-deployment-manager.naffets.eu,resources=taskdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TaskDefinition object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *TaskDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var taskDefinition ecsdeploymentmanagerv1alpha1.TaskDefinition
	if err := r.Get(ctx, req.NamespacedName, &taskDefinition); err == nil {
		reconcilerClient := reconcilers.NewTaskDefinitionReconcilerClient(r.Client, &taskDefinition)
		return reconcilerClient.Reconcile(ctx), nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TaskDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ecsdeploymentmanagerv1alpha1.TaskDefinition{}).
		Complete(r)
}
