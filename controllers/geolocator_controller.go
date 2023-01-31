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
	Status      string  `json:"status"`
	Message     string  `json:"message,omitempty"`
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

// See https://ip-api.com
const locationApi = "http://ip-api.com"

//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.suse.com,resources=geolocators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the GeoLocator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *GeoLocatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := log.FromContext(ctx)

	Log.Info("Reconcile cycle")

	nodes := &corev1.NodeList{}
	err := r.Client.List(ctx, nodes)
	if err != nil {
		Log.Error(err, "Failed to get Nodes list")
		return ctrl.Result{}, err
	}

	for i := 0; i < len(nodes.Items); i++ {
		node := nodes.Items[i].DeepCopy()
		address := filterAddressByType(node.Status.Addresses, "InternalIP")[0]

		location, err := getNodeLocation(address.Address)
		if err != nil {
			Log.Error(err, "Unable to get Node location")
			return ctrl.Result{}, err
		}

		lat := fmt.Sprintf("%.3f", location.Lat)
		lon := fmt.Sprintf("%.3f", location.Lon)

		if node.Annotations["geo-locator.lat"] != lat || node.Annotations["geo-locator.lon"] != lon {

			node.Annotations["geo-locator.country"] = location.Country
			node.Annotations["geo-locator.region"] = location.RegionName
			node.Annotations["geo-locator.city"] = location.City
			node.Annotations["geo-locator.lat"] = lat
			node.Annotations["geo-locator.lon"] = lon

			Log.Info("Updating Node location info")
			updates := client.MergeFrom(&nodes.Items[i])
			if err := r.Client.Patch(ctx, node, updates); err != nil {
				Log.Error(err, "Failed to update Node")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func getNodeLocation(ip string) (NodeLocation, error) {

	location, err := ipLocationRequest(ip)
	if err != nil {
		return NodeLocation{}, err
	}

	// Internal ip (localhost)
	if location.Status == "fail" {
		location, err = ipLocationRequest("")
		if err != nil {
			return NodeLocation{}, err
		}
	}

	return location, nil
}

func ipLocationRequest(ip string) (NodeLocation, error) {

	url := fmt.Sprintf("%s/json", locationApi)
	if len(ip) > 0 {
		url = fmt.Sprintf("%s/%s", url, ip)
	}

	resp, err := http.Get(url)
	if err != nil {
		return NodeLocation{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var location NodeLocation
	if err := json.Unmarshal(body, &location); err != nil {
		return NodeLocation{}, err
	}

	return location, nil
}

func filterAddressByType(na []corev1.NodeAddress, t corev1.NodeAddressType) (out []corev1.NodeAddress) {
	for _, a := range na {
		if a.Type == t {
			out = append(out, a)
		}
	}
	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *GeoLocatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}). // Watch Nodes update
		Complete(r)
}
