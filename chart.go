package kubby

import (
	"fmt"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
)

type HelmChart struct {
	Path      string
	Namespace string
	Name      string
}

type ChartMap map[string]*HelmChart

type HelmChartManager struct {
	KubeConfigPath string
	Charts         ChartMap
}

func NewHelmChartManager(path string) (*HelmChartManager, error) {
	hcm := &HelmChartManager{
		KubeConfigPath: path,
		Charts:         ChartMap{},
	}

	return hcm, nil
}

func (hcm *HelmChartManager) InstallChart(name string, namespace string, path string) error {
	fmt.Printf("installing %s chart ...\n", name)
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(kube.GetConfig(hcm.KubeConfigPath, "", namespace), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Printf(format, v)
		fmt.Println()
	})
	if err != nil {
		return fmt.Errorf("InstallChart: %w", err)
	}

	chart, err := loader.Load(path)
	if err != nil {
		return fmt.Errorf("InstallChart: %w", err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = namespace
	client.ReleaseName = name

	_, err = client.Run(chart, nil)
	if err != nil {
		return fmt.Errorf("InstallChart: %w", err)
	}

	hcm.Charts[name] = &HelmChart{
		Namespace: namespace,
		Name:      name,
		Path:      path,
	}

	return nil
}
