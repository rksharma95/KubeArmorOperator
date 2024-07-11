// Package helm ...
package helm

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"

	semver "github.com/Masterminds/semver/v3"
	operatorv1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/api/v1"
	embedFs "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/embed"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
)

var (
	settings     = cli.New()
	actionConfig = &action.Configuration{}
)

// Config provides configurations to initialize a helm controller instance
type Config struct {
	// chartRef or chart name
	ChartName string
	// namespace to deploy chart
	Namespace string
	// chart version to install
	Version string
	// chart repository
	Repository string
	// chart directory if local chart
	Directory string
}

// Controller contains helm chart configurations
type Controller struct {
	mutex sync.Mutex
	// Helm release chartName
	chartName string
	// Helm release namespace
	namespace string
	// Helm chart
	chart *chart.Chart
	// Helm values generated using kubearmorconfig instance
	kaConfigValues map[string]interface{}
	// Helm values generated using node configuration
	nodeConfigValues map[string]interface{}
}

// NewHelmController creates an instance of helm controller using provided configurations
// and return it on successful initialization otherwise returns an error
func NewHelmController(cfg Config) (*Controller, error) {
	err := actionConfig.Init(settings.RESTClientGetter(), cfg.Namespace, os.Getenv("HELM_DRIVER"), log.Printf)
	if err != nil {
		return nil, fmt.Errorf("error initializing helm action config: %s", err.Error())
	}
	chart, err := GetHelmChart(cfg.Repository, cfg.Version, cfg.Directory, cfg.ChartName)
	if err != nil {
		return nil, fmt.Errorf("error pulling helm chart: %s", err.Error())
	}

	log.Printf("helm controller has configured: %+v", cfg)

	return &Controller{
		mutex:            sync.Mutex{},
		chartName:        cfg.ChartName,
		namespace:        cfg.Namespace,
		chart:            chart,
		kaConfigValues:   map[string]interface{}{},
		nodeConfigValues: map[string]interface{}{},
	}, nil
}

type resource struct {
	kind  string
	name  string
	group string
}

// UpdateHelmValuesFromKubeArmorConfig function merge helm values with new values
// defined with kubearmorconfig instance
func (ctrl *Controller) UpdateHelmValuesFromKubeArmorConfig(kaConfig *operatorv1.KubeArmorConfig) {
	kaConfigHelmValues := map[string]interface{}{}

	// configmapvalues => Values.kubearmorConfigMap
	configMapValues := map[string]interface{}{}
	kaConfigHelmValues["kubearmorConfigMap"] = configMapValues
	// default postures
	if val := kaConfig.Spec.DefaultFilePosture; val != "" {
		configMapValues["defaultFilePosture"] = string(val)
	}
	if val := kaConfig.Spec.DefaultCapabilitiesPosture; val != "" {
		configMapValues["defaultCapabilitiesPosture"] = string(val)
	}
	if val := kaConfig.Spec.DefaultNetworkPosture; val != "" {
		configMapValues["defaultNetworkPosture"] = string(val)
	}
	// default visibility
	if val := kaConfig.Spec.DefaultVisibility; val != "" {
		configMapValues["visibility"] = string(val)
	}
	// alert throttling
	configMapValues["alertThrottling"] = strconv.FormatBool(kaConfig.Spec.AlertThrottling)
	if val := kaConfig.Spec.MaxAlertPerSec; val != 0 {
		configMapValues["maxAlertPerSec"] = val
	}
	if val := kaConfig.Spec.ThrottleSec; val != 0 {
		configMapValues["throttleSec"] = val
	}

	// tls => Values.tls
	kaConfigHelmValues["tls"] = map[string]interface{}{
		"enabled": kaConfig.Spec.Tls.Enable,
	}
	// kubearmor-relay => Values.kubearmorRelay
	relay := map[string]interface{}{}
	kaConfigHelmValues["kubearmorRelay"] = relay
	// relay tls configurations
	relayTLS := map[string]interface{}{}
	relay["tls"] = relayTLS
	if val := kaConfig.Spec.Tls.RelayExtraDnsNames; len(val) > 0 {
		relayTLS["extraDnsNames"] = val
	}
	if val := kaConfig.Spec.Tls.RelayExtraIpAddresses; len(val) > 0 {
		relayTLS["extraIpAddresses"] = val
	}
	// relay image
	relayImage := map[string]interface{}{}
	relay["image"] = relayImage
	if val := kaConfig.Spec.KubeArmorRelayImage; !val.IsEmpty() {
		if val.Image != "" {
			imageAndTag := strings.Split(val.Image, ":")
			image := imageAndTag[0]
			relayImage["repository"] = image
			if len(imageAndTag) > 1 {
				relayImage["tag"] = imageAndTag[1]
			}
		}
		if val.ImagePullPolicy != "" {
			relay["imagePullPolicy"] = val.ImagePullPolicy
		}
	}
	// relay env vars
	relay["enableStdoutLogs"] = strconv.FormatBool(kaConfig.Spec.EnableStdOutLogs)
	relay["enableStdoutAlerts"] = strconv.FormatBool(kaConfig.Spec.EnableStdOutAlerts)
	relay["enableStdoutMsg"] = strconv.FormatBool(kaConfig.Spec.EnableStdOutMsgs)
	// handle seccomp

	ctrl.kaConfigValues = kaConfigHelmValues
}

func (ctrl *Controller) UpdateNodeConfigHelmValues(nodeConfig []map[string]interface{}) {
	ctrl.nodeConfigValues = map[string]interface{}{
		"nodes": nodeConfig,
	}
}

func pullHelmChartFromOCIRegistry(repository, version, chart, targetDir string) (*chart.Chart, error) {
	// create a temp subdirectory to pull helm chart
	file := path.Join(targetDir, fmt.Sprintf("%s-%s.tgz", chart, version))
	actionCfg := &action.Configuration{}
	pull := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pull.Settings = settings
	pull.Version = version
	pull.DestDir = targetDir
	// in case of private registries ??
	client, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	actionCfg.RegistryClient = client
	_, err = pull.Run(repository)
	if err != nil {
		return nil, err
	}
	// load pulled helm chart from archieve file
	return loader.Load(file)
}

// GetHelmChart pull helm chart from given helm parameters
func GetHelmChart(repository, version, directory, chartName string) (*chart.Chart, error) {
	// TODO: validate chart version ^v1.3.8
	// check if local helm chart is to be used
	if directory != "" {
		chart, err := loader.Load(directory)
		if err != nil {
			return nil, err
		}
		return chart, nil
	}

	if repository == "embed" {
		chartArchieve, err := embedFs.EmbedFs.ReadFile(fmt.Sprintf("%s-%s.tgz", chartName, version))
		if err != nil {
			return nil, err
		}
		return loader.LoadArchive(bytes.NewReader(chartArchieve))
	}

	// create a cache directory to store pulled helm chart
	targetDir := path.Join(os.TempDir(), "kubearmor", ".cache")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil && !os.IsExist(err) {
		targetDir = "./"
	}

	if registry.IsOCI(repository) {
		return pullHelmChartFromOCIRegistry(repository, version, chartName, targetDir)
	}

	file := path.Join(targetDir, fmt.Sprintf("%s-%s.tgz", chartName, version))
	pull := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pull.Settings = settings
	pull.Version = version
	pull.RepoURL = repository
	pull.DestDir = targetDir
	// pull chart
	_, err = pull.Run(chartName)
	if err != nil {
		log.Printf("error pulling helm chart: %s", err.Error())
		return nil, err
	}
	// load pulled helm chart from archieve file
	return loader.Load(file)
}

// checkIfCleanUpRequired check for recent two revisions of (if any) existing
// kubearmor-operator release and check if last installed version is <v1.3.8
func checkIfCleanUpRequired() bool {
	v138, _ := semver.NewVersion("v1.3.8")
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 10
	release, err := histClient.Run("kubearmor-operator")
	if err != nil && err == driver.ErrReleaseNotFound {
		return false
	}
	for _, rel := range release {
		ver, _ := semver.NewVersion(rel.Chart.Metadata.Version)
		if ver.Equal(v138) {
			continue
		} else if ver.LessThan(v138) {
			return true
		} else if ver.GreaterThan(v138) {
			return false
		}
	}
	return false
}

func uninstallRelease(releaseName string) error {
	uninstallClient := action.NewUninstall(actionConfig)
	_, err := uninstallClient.Run(releaseName)
	if err != nil {
		return err
	}
	return nil
}

func removeManifestHeader(manifest string) string {
	var cleanedLines []string
	lines := strings.Split(manifest, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}
	// log.Printf("resource after header removed: \n%s\n", strings.Join(cleanedLines, "\n"))
	return strings.Join(cleanedLines, "\n")
}

func (ctrl *Controller) cleanUpResources(ctx context.Context, actionConfig *action.Configuration) ([]resource, error) {
	installClient := action.NewInstall(actionConfig)
	installClient.Namespace = ctrl.namespace
	installClient.ReleaseName = ctrl.chartName
	installClient.ClientOnly = true
	installClient.DryRun = true

	rel, _ := installClient.RunWithContext(ctx, ctrl.chart, ctrl.kaConfigValues)

	var resources []resource
	if rel != nil {
		manifests := releaseutil.SplitManifests(rel.Manifest)
		for _, manifest := range manifests {
			cleanManifest := removeManifestHeader(manifest)
			if strings.TrimSpace(cleanManifest) == "" {
				continue
			}
			u := unstructured.Unstructured{}
			jsonData, err := yaml.YAMLToJSON([]byte(cleanManifest))
			if err != nil {
				return nil, fmt.Errorf("error converting YAML to JSON: %v", err)
			}
			_, _, err = unstructured.UnstructuredJSONScheme.Decode([]byte(jsonData), nil, &u)
			if err != nil {
				return nil, fmt.Errorf("error decoding manifest: %v", err)
			}

			resources = append(resources, resource{
				kind:  u.GetKind(),
				name:  u.GetName(),
				group: u.GroupVersionKind().Group,
			})
		}
		log.Printf("list of resources to clean: %d\n", len(resources))
	}
	return resources, nil
}

// Preinstall checks if previous operator was older than v1.3.4, in that case it requires deleting the KubeArmor k8s
// resources to be deleted explicitly to avoid conflict between controller that manages resources as helm need to be
// the controller to manage KubeArmor k8s resources
func (ctrl *Controller) Preinstall() error {
	err := actionConfig.Init(settings.RESTClientGetter(), ctrl.namespace, "", func(format string, v ...interface{}) {})
	if err != nil {
		log.Fatalf("error initializing action config: %s", err.Error())
	}
	config, err := settings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return err
	}

	required := checkIfCleanUpRequired()
	if !required {
		return nil
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	discoveryClient, err := settings.RESTClientGetter().ToDiscoveryClient()
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	resources, err := ctrl.cleanUpResources(context.Background(), actionConfig)
	if err != nil {
		log.Printf("error getting resources: %s\n", err.Error())
	}
	for _, resource := range resources {
		mapping, err := mapper.RESTMapping(schema.GroupKind{Group: resource.group, Kind: resource.kind})
		if err != nil {
			fmt.Printf("failed to get mapping to kind: %s: %s\n", resource.kind, err.Error())
			continue
		}
		resourceClient := dynamicClient.Resource(mapping.Resource).Namespace(ctrl.namespace)
		err = resourceClient.Delete(context.TODO(), resource.name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete %s %s: %v", resource.kind, resource.name, err)
		}
		fmt.Printf("Successfully deleted %s: %s\n", resource.kind, resource.name)
	}

	// === handle kubearmor daemonset and controller seperately ===

	// GVR for daemonsets
	dsGvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "daemonsets",
	}
	daemonSetClient := dynamicClient.Resource(dsGvr).Namespace(ctrl.namespace)
	daemonSets, err := daemonSetClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: "kubearmor-app=kubearmor",
	})
	if err != nil {
		log.Printf("failed to list kubearmor daemonsets error=%s", err.Error())
		return err
	}
	for _, ds := range daemonSets.Items {
		err := daemonSetClient.Delete(context.Background(), ds.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Printf("error deleteing daemonset %s error=%s", ds.GetName(), err.Error())
			return err
		}
		fmt.Printf("Successfully deleted %s: %s\n", ds.GetKind(), ds.GetName())
	}

	// GVR for deployments
	depGvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	deployClient := dynamicClient.Resource(depGvr).Namespace(ctrl.namespace)
	err = deployClient.Delete(context.Background(), "kubearmor-controller", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete deployment kubearmor-controller %s", err.Error())
	}
	fmt.Printf("Successfully deleted %s: %s\n", "Deployment", "kubearmor-controller")
	return nil
}

// UpgradeRelease performs helm upgrade for helm chart defined with configuration
func (ctrl *Controller) UpgradeRelease(ctx context.Context) (*release.Release, error) {
	ctrl.mutex.Lock()
	defer ctrl.mutex.Unlock()

	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	release, err := histClient.Run(ctrl.chartName)

	var vals map[string]interface{}
	vals = mergeMaps(ctrl.kaConfigValues, ctrl.nodeConfigValues)

	// Not a best way to sync between kubearmorconfig reconiler and clusterwatcher
	// to check and deploy KubeArmor applications only if snitch detected node configuration
	// and kubearmoconfig CR instance has been detected
	if len(ctrl.kaConfigValues) < 1 || len(ctrl.nodeConfigValues) < 1 {
		return nil, fmt.Errorf("either nodes are not processed or kubearmorconfig CR instance not present")
	}

	fmt.Printf("vals: %+n", vals)

	if err != nil && err == driver.ErrReleaseNotFound {
		fmt.Println("no existing kubearmor release installing now")
		// release not found install release
		installClient := action.NewInstall(actionConfig)
		if installClient == nil {
			return nil, fmt.Errorf("unable to create install client")
		}
		installClient.Namespace = ctrl.namespace
		installClient.ReleaseName = ctrl.chartName
		installClient.Wait = true
		installClient.Timeout = 5 * time.Minute
		// installClient.Atomic = true
		// return installClient.RunWithContext(ctx, ctrl.chart, vals)
		return installClient.Run(ctrl.chart, vals)
	}
	fmt.Println("found existing kubearmor release upgrading now")
	if release[0].Info.Status != "deployed" {
		return nil, fmt.Errorf("previous release status is not deployed but %s", release[0].Info.Status)
	}
	upgradeClient := action.NewUpgrade(actionConfig)
	// upgradeClient.Atomic = true
	upgradeClient.ResetValues = true
	upgradeClient.Wait = true
	upgradeClient.Timeout = 5 * time.Minute
	upgradeClient.Namespace = ctrl.namespace
	return upgradeClient.RunWithContext(ctx, ctrl.chartName, ctrl.chart, vals)
}

// mergeMaps
// https://pkg.go.dev/helm.sh/helm/v3@v3.15.2/pkg/cli/values#Options.MergeValues
func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
