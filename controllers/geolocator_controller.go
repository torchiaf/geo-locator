/*
Copyright 2023.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GeoLocatorReconciler reconciles a GeoLocator object
type GeoLocatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type NodeLocation struct {
	Query       string  `json:"query"`
	Status      string  `json:"status,omitempty"`
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"countryCode,omitempty"`
	Region      string  `json:"region,omitempty"`
	RegionName  string  `json:"regionName,omitempty"`
	City        string  `json:"city,omitempty"`
	Zip         string  `json:"zip,omitempty"`
	Lat         float32 `json:"lat,omitempty"`
	Lon         float32 `json:"lon,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	Isp         string  `json:"isp,omitempty"`
	Org         string  `json:"org,omitempty"`
	As          string  `json:"as,omitempty"`
}

//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by

// compare nodes to see if labels are null, then update

// the GeoLocator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *GeoLocatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := log.FromContext(ctx)

	Log.Info("Reconcile cycle")

	// app := &appsv1.GeoLocator{}
	// err := r.Client.Get(ctx, req.NamespacedName, app)
	// if err != nil {
	// 	Log.Error(err, "Failed to get GeoLocator App")
	// 	return ctrl.Result{}, err
	// }
	// Log.Info(fmt.Sprintf("Reconcile spec: %t", app.Spec.Labeled))

	nodes := &corev1.NodeList{}
	err := r.Client.List(ctx, nodes)
	if err != nil {
		Log.Error(err, "Failed to get Nodes list")
		return ctrl.Result{}, err
	}

	for i := 0; i < len(nodes.Items); i++ {
		node := nodes.Items[i]
		address := node.Status.Addresses[0].Address

		infoMessage := fmt.Sprintf("Node Ip found: %s", address)
		Log.Info(infoMessage)

		// See https://ip-api.com
		resp, err := http.Get(`http://ip-api.com/json/`)

		if err != nil {
			Log.Error(err, "Failed to get Node position")
			return ctrl.Result{}, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		var location NodeLocation
		if err := json.Unmarshal(body, &location); err != nil {
			fmt.Println("Can not unmarshal JSON position")
			return ctrl.Result{}, err
		}

		node.Annotations["geo-locator.country"] = location.Country
		node.Annotations["geo-locator.region"] = location.RegionName
		node.Annotations["geo-locator.city"] = location.City

		if err := r.Update(ctx, &node); err != nil {
			Log.Error(err, "unable to update Node")
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GeoLocatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}). // Watch Nodes update
		Complete(r)
}
