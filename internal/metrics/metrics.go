package metrics

import (
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MetricsValues struct {
	Enabled               bool   `json:"enabled"`
	AddonInstallNamespace string `json:"addonInstallNamespace"`
}

func GetValuesFunc(_ client.Client, _ *clusterv1.ManagedCluster, mca *addonapiv1alpha1.ManagedClusterAddOn) (MetricsValues, error) {
	values := MetricsValues{
		Enabled:               true,
		AddonInstallNamespace: mca.Spec.InstallNamespace,
	}
	return values, nil
}
