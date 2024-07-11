package main

import (
	"context"
	// "encoding/json"
	"log"

	operatorv1 "github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/api/v1"
	"github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/internal/helm"
)

func main() {

	helmConfig := helm.Config{
		// ChartName:  "kubearmor",
		// Namespace:  "kubearmor",
		// Version:    "v1.3.4",
		// Repository: "https://kubearmor.github.io/charts",
		// Directory:  "/home/hp/Documents/KubeArmor/deployments/helm/KubeArmor",
		ChartName:  "kubearmor",
		Namespace:  "kubearmor",
		Version:    "v1.3.8",
		Repository: "embed",
		Directory:  "",
	}

	// chart, err := helm.GetHelmChart(helmConfig.Repository, helmConfig.Version, helmConfig.Directory, helmConfig.ChartName)
	// if err != nil {
	// 	log.Fatalf("unabel to pull helm chart error=%s", err.Error())
	// }
	// log.Printf("pulled helm chart %s:%s", chart.Name(), chart.Metadata.Version)
	helmCtrl, err := helm.NewHelmController(helmConfig)
	if err != nil {
		log.Fatalf("unable to initialize helm controller error=%s", err.Error())
	}

	if err := helmCtrl.Preinstall(); err != nil {
		log.Fatalf("unable to cleanup existing release error=%s", err.Error())
	}

	helmCtrl.UpdateHelmValuesFromKubeArmorConfig(&operatorv1.KubeArmorConfig{
		Spec: operatorv1.KubeArmorConfigSpec{
			DefaultFilePosture:         "block",
			DefaultCapabilitiesPosture: "block",
			DefaultNetworkPosture:      "block",
			DefaultVisibility:          "process,file,network",
			KubeArmorRelayImage: operatorv1.ImageSpec{
				Image: "kubearmor/kubearmor-relay-server:latest",
			},
			AlertThrottling: true,
		},
	})

	helmCtrl.UpdateNodeConfigHelmValues([]map[string]interface{}{
		{
			"config": map[string]interface{}{
				"enforcer":   "apparmor",
				"runtime":    "containerd",
				"socket":     "run_containerd_containerd.sock",
				"arch":       "amd64",
				"btf":        "yes",
				"apparmorfs": "yes",
				"seccomp":    "yes",
			},
		},
	})

	release, err := helmCtrl.UpgradeRelease(context.Background())
	if err != nil {
		log.Fatalf("unable to upgrade release error=%s", err.Error())
	}
	log.Printf("upgraded release %s in namespace %s", release.Name, release.Namespace)

	// return

	// chart, err := helm.GetHelmChart("", "", "/home/hp/Documents/KubeArmor/deployments/helm/KubeArmor", "")
	// if err != nil {
	// 	log.Fatalf("unable to get helm chart error=%s", err.Error())
	// }
	// log.Printf("chart %s loaded", chart.Name())
	// log.Printf("chart values %+v", chart.Values)

	// return

	// helmController, err := helm.NewHelmController(helmConfig)
	// if err != nil {
	// 	log.Fatalf("error initializing helm controller")
	// }

	// err = helmController.Preinstall()
	// if err != nil {
	// 	log.Fatalf("unable to did cleanup: %s", err.Error())
	// }

	// return

	// kaConfig := &operatorv1.KubeArmorConfig{
	// 	Spec: operatorv1.KubeArmorConfigSpec{
	// 		DefaultFilePosture:         "block",
	// 		DefaultCapabilitiesPosture: "block",
	// 		DefaultNetworkPosture:      "block",
	// 		DefaultVisibility:          "process,file,network",
	// 		KubeArmorRelayImage: operatorv1.ImageSpec{
	// 			Image: "kubearmor/kubearmor-relay-server:v1.3.8",
	// 		},
	// 		AlertThrottling: true,
	// 	},
	// }

	// if kaConfig.Spec.KubeArmorImage.IsEmpty() {
	// 	log.Print("config has no kubearmor image")
	// }

	// jsoneData, err := json.Marshal(kaConfig)
	// if err != nil {
	// 	log.Printf("error marshalling kaConfig: %s\n", err.Error())
	// }
	// log.Print(string(jsoneData))

	// // log.Printf("kubearmor helm values: \n%+v\n", helmConfig.Values)
	// helmController.UpdateHelmValuesFromKubeArmorConfig(kaConfig)
	// // log.Printf("helm values after update: \n%+v\n", helmConfig.Values)

	// release, err = helmController.UpgradeRelease(context.TODO())
	// if err != nil {
	// 	log.Fatalf("failed to upgrade release: %s\n", err.Error())
	// }
	// log.Printf("upgrade successful for release: %s", release.Name)
}
