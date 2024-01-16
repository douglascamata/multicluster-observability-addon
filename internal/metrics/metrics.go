package metrics

import (
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetricsValues struct {
	Enabled bool `json:"enabled"`
}

func GetValuesFunc(_ client.Client, _ *clusterv1.ManagedCluster, _ *addonapiv1alpha1.ManagedClusterAddOn) (MetricsValues, error) {
	values := MetricsValues{
		Enabled: false,
	}

	values.Enabled = true
	return values, nil
}
