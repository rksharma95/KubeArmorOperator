package controller

import (
	"os"

	"github.com/kubearmor/KubeArmor/pkg/KubeArmorOperator/internal/helm"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// OperatorConfig injects operator configurations
type OperatorConfig struct {
	// kubearmor chart version to install
	Version string
	// kubearmor chart repository
	Repository string
	// chart directory if local chart
	Directory string
	// chart name or chartRef
	ChartName string
	// namespace to deploy chart
	Namespace string
	// Snitch path prefix
	SnitchPathPrefix string
	// Operator deployment name
	OperatorDeploymentName string
	// operator deployment uid
	OperatorDeploymentUID string
}

// Operator repesents operator implementation
type Operator struct {
	k8sClient                 client.Client
	k8sClientSet              *kubernetes.Clientset
	log                       *zap.SugaredLogger
	clusterWatcher            *ClusterWatcher
	helmInstaller             *helm.Controller
	kubeArmorConfigReconciler *KubeArmorConfigReconciler
	controllerManager         ctrl.Manager
}

// NewOperator initializes and returns an operator instance
func NewOperator(cfg OperatorConfig, k8sClient client.Client, k8sClientSet *kubernetes.Clientset, manager ctrl.Manager) (*Operator, error) {
	logger, _ := zap.NewProduction()
	log := logger.With(zap.String("component", "operator")).Sugar()

	log.Infof("operator has been configured %+v", cfg)

	// helm controller
	helmConfig := helm.Config{
		ChartName:  cfg.ChartName,
		Namespace:  cfg.Namespace,
		Version:    cfg.Version,
		Repository: cfg.Repository,
		Directory:  cfg.Directory,
	}

	helmController, err := helm.NewHelmController(helmConfig)
	if err != nil {
		return nil, err
	}

	// cluster watcher
	watcherConfig := WatcherConfig{
		SnitchPathPrefix:         cfg.SnitchPathPrefix,
		OperatorWatchedNamespace: cfg.Namespace,
		OperatorDeploymentName:   cfg.OperatorDeploymentName,
		OperatorDeploymentUID:    cfg.OperatorDeploymentUID,
	}

	clusterWatcher, err := NewClusterWatcher(watcherConfig, k8sClientSet, helmController)
	if err != nil {
		return nil, err
	}
	kubeArmorConfigReconciler := KubeArmorConfigReconciler{
		helmController,
		k8sClient,
		k8sClient.Scheme(),
	}

	return &Operator{
		k8sClient,
		k8sClientSet,
		log,
		clusterWatcher,
		helmController,
		&kubeArmorConfigReconciler,
		manager,
	}, nil
}

// Start runs operator componenets
func (operator *Operator) Start() {
	err := operator.helmInstaller.Preinstall()
	if err != nil {
		operator.log.Errorf("error while cleaning up existing release", err.Error())
	}

	// start cluster(node)watcher
	go operator.clusterWatcher.WatchNodes()

	// start kubeconfigreconciler
	if err = operator.kubeArmorConfigReconciler.SetupWithManager(operator.controllerManager); err != nil {
		operator.log.Error(err, "unable to create controller", "controller", "KubeArmorConfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := operator.controllerManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		operator.log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := operator.controllerManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		operator.log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	operator.log.Info("starting manager")
	if err := operator.controllerManager.Start(ctrl.SetupSignalHandler()); err != nil {
		operator.log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
