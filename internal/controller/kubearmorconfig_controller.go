/*
Copyright 2024.

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

package controller

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	operatorv1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/api/v1"
	helm "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/internal/helm"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// KubeArmorConfigReconciler reconciles a KubeArmorConfig object
type KubeArmorConfigReconciler struct {
	helmController *helm.Controller
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.kubearmor.com,resources=kubearmorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kubearmor.com,resources=kubearmorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kubearmor.com,resources=kubearmorconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KubeArmorConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *KubeArmorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	config := &operatorv1.KubeArmorConfig{}
	// TODO(user): your logic here
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !config.GetDeletionTimestamp().IsZero() {
		// kubearmorconfig CR instance has been deleted
	}
	// update helm values from KubeArmorConfig CR instance
	// do helm upgrade
	logger.Info("upgrading release with kubearmorconfig changes")
	r.helmController.UpdateHelmValuesFromKubeArmorConfig(config)
	release, err := r.helmController.UpgradeRelease(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "nodes are not processed or kubearmorconfig") {
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("successfully upgraded release", "name", release.Name, "version", release.Version)
	logger.Info("release status info", "status", release.Info.Status, "chartVersion", release.Chart.Metadata.Version)
	if release != nil {
		var manifests bytes.Buffer
		fmt.Fprintf(&manifests, strings.TrimSpace(release.Manifest))
		for _, m := range release.Hooks {
			fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
		}

		fmt.Println(manifests.String())
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeArmorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.KubeArmorConfig{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(ue event.UpdateEvent) bool {
				oldConfig := ue.ObjectOld.(*operatorv1.KubeArmorConfig)
				newConfig := ue.ObjectNew.(*operatorv1.KubeArmorConfig)
				return !reflect.DeepEqual(oldConfig.Spec, newConfig.Spec)
			},
		}).
		Complete(r)
}
