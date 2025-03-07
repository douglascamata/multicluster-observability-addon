package logging

import (
	"fmt"
	"testing"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	"github.com/rhobs/multicluster-observability-addon/internal/addon"

	loggingapis "github.com/openshift/cluster-logging-operator/apis"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager/addontesting"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	fakeaddon "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	_ = loggingapis.AddToScheme(scheme.Scheme)
	_ = operatorsv1.AddToScheme(scheme.Scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme.Scheme)
)

func fakeGetValues(k8s client.Client) addonfactory.GetValuesFunc {
	return func(
		cluster *clusterv1.ManagedCluster,
		addon *addonapiv1alpha1.ManagedClusterAddOn,
	) (addonfactory.Values, error) {
		logging, err := GetValuesFunc(k8s, cluster, addon, nil)
		if err != nil {
			return nil, err
		}

		return addonfactory.JsonStructToValues(logging)
	}
}

func Test_Logging_SubscriptionChannel(t *testing.T) {
	var (
		managedCluster        *clusterv1.ManagedCluster
		managedClusterAddOn   *addonapiv1alpha1.ManagedClusterAddOn
		addOnDeploymentConfig *addonapiv1alpha1.AddOnDeploymentConfig
	)

	managedCluster = addontesting.NewManagedCluster("cluster-1")
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")

	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management",
				Name:      "multicluster-observability-addon",
			},
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "loggingSubscriptionChannel",
					Value: "stable-5.8",
				},
			},
		},
	}

	fakeAddonClient := fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 6, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, obj.Spec.Channel, "stable-5.8")
		}
	}
}

func Test_Logging_AllConfigsTogether_AllResources(t *testing.T) {
	var (
		// Addon envinronment and registration
		managedCluster      *clusterv1.ManagedCluster
		managedClusterAddOn *addonapiv1alpha1.ManagedClusterAddOn

		// Addon configuration
		addOnDeploymentConfig            *addonapiv1alpha1.AddOnDeploymentConfig
		clf                              *loggingv1.ClusterLogForwarder
		appLogsSecret, clusterLogsSecret *corev1.Secret
		appLogsCm                        *corev1.ConfigMap

		// Test clients
		fakeKubeClient  client.Client
		fakeAddonClient *fakeaddon.Clientset
	)

	// Setup a managed cluster
	managedCluster = addontesting.NewManagedCluster("cluster-1")

	// Register the addon for the managed cluster
	managedClusterAddOn = addontesting.NewAddon("test", "cluster-1")
	managedClusterAddOn.Status.ConfigReferences = []addonapiv1alpha1.ConfigReference{
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management",
				Name:      "multicluster-observability-addon",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "logging.openshift.io",
				Resource: "clusterlogforwarders",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: "open-cluster-management",
				Name:      "instance",
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: managedCluster.Name,
				Name:      fmt.Sprintf("%s-app-logs", managedCluster.Name),
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "secrets",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: managedCluster.Name,
				Name:      fmt.Sprintf("%s-cluster-logs", managedCluster.Name),
			},
		},
		{
			ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
				Group:    "",
				Resource: "configmaps",
			},
			ConfigReferent: addonapiv1alpha1.ConfigReferent{
				Namespace: managedCluster.Name,
				Name:      fmt.Sprintf("%s-app-logs", managedCluster.Name),
			},
		},
	}

	// Setup configuration resources: ClusterLogForwarder, AddOnDeploymentConfig, Secrets, ConfigMaps
	clf = &loggingv1.ClusterLogForwarder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instance",
			Namespace: "open-cluster-management",
		},
		Spec: loggingv1.ClusterLogForwarderSpec{
			Inputs: []loggingv1.InputSpec{
				{
					Name: "app-logs",
					Application: &loggingv1.Application{
						Namespaces: []string{"ns-1", "ns-2"},
					},
				},
				{
					Name:           "infra-logs",
					Infrastructure: &loggingv1.Infrastructure{},
				},
			},
			Outputs: []loggingv1.OutputSpec{
				{
					Name: "app-logs",
					Type: loggingv1.OutputTypeLoki,
					OutputTypeSpec: loggingv1.OutputTypeSpec{
						Loki: &loggingv1.Loki{
							LabelKeys: []string{"key-1", "key-2"},
							TenantKey: "tenant-x",
						},
					},
				},
				{
					Name: "cluster-logs",
					Type: loggingv1.OutputTypeCloudwatch,
					OutputTypeSpec: loggingv1.OutputTypeSpec{
						Cloudwatch: &loggingv1.Cloudwatch{
							GroupBy:     loggingv1.LogGroupByLogType,
							GroupPrefix: pointer.String("test-prefix"),
						},
					},
				},
			},
			Pipelines: []loggingv1.PipelineSpec{
				{
					Name:       "app-logs",
					InputRefs:  []string{"app-logs"},
					OutputRefs: []string{"app-logs"},
				},
				{
					Name:       "cluster-logs",
					InputRefs:  []string{"infra-logs", loggingv1.InputNameAudit},
					OutputRefs: []string{"cluster-logs"},
				},
			},
		},
	}

	appLogsSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-app-logs", managedCluster.Name),
			Namespace: managedCluster.Name,
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
			Annotations: map[string]string{
				annotationTargetOutputName: "app-logs",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		},
	}

	clusterLogsSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-cluster-logs", managedCluster.Name),
			Namespace: managedCluster.Name,
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
			Annotations: map[string]string{
				annotationTargetOutputName: "cluster-logs",
			},
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		},
	}

	appLogsCm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-app-logs", managedCluster.Name),
			Namespace: managedCluster.Name,
			Labels: map[string]string{
				"mcoa.openshift.io/signal": "logging",
			},
			Annotations: map[string]string{
				annotationTargetOutputName: "app-logs",
			},
		},
		Data: map[string]string{
			"url": "https://example.com",
		},
	}

	addOnDeploymentConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-observability-addon",
			Namespace: "open-cluster-management",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "loggingSubscriptionChannel",
					Value: "stable-5.8",
				},
			},
		},
	}

	// Setup the fake k8s client
	fakeKubeClient = fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(clf, appLogsCm, appLogsSecret, clusterLogsSecret).
		Build()

	// Setup the fake addon client
	fakeAddonClient = fakeaddon.NewSimpleClientset(addOnDeploymentConfig)
	addonConfigValuesFn := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeploymentConfigGetter(fakeAddonClient),
		addonfactory.ToAddOnCustomizedVariableValues,
	)

	// Wire everything together to a fake addon instance
	loggingAgentAddon, err := addonfactory.NewAgentAddonFactory(addon.Name, addon.FS, addon.LoggingChartDir).
		WithGetValuesFuncs(addonConfigValuesFn, fakeGetValues(fakeKubeClient)).
		WithAgentRegistrationOption(&agent.RegistrationOption{}).
		WithScheme(scheme.Scheme).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Fatalf("failed to build agent %v", err)
	}

	// Render manifests and return them as k8s runtime objects
	objects, err := loggingAgentAddon.Manifests(managedCluster, managedClusterAddOn)
	require.NoError(t, err)
	require.Equal(t, 6, len(objects))

	for _, obj := range objects {
		switch obj := obj.(type) {
		case *operatorsv1alpha1.Subscription:
			require.Equal(t, obj.Spec.Channel, "stable-5.8")
		case *loggingv1.ClusterLogForwarder:
			require.NotNil(t, obj.Spec.Outputs[0].Secret)
			require.NotNil(t, obj.Spec.Outputs[1].Secret)
			require.Equal(t, appLogsSecret.Name, obj.Spec.Outputs[0].Secret.Name)
			require.Equal(t, clusterLogsSecret.Name, obj.Spec.Outputs[1].Secret.Name)
			require.Equal(t, appLogsCm.Data["url"], obj.Spec.Outputs[0].URL)
		}
	}
}
