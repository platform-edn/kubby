package kubby

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

type KubeResourcer interface {
	RunJob(context.Context, string, *batchv1.Job, time.Duration) error
	CreateDeployment(context.Context, string, *appsv1.Deployment) error
	DeleteDeployment(context.Context, string, string) error
}

type KubeCluster struct {
	Provider         *cluster.Provider
	Name             string
	KubeConfigPath   string
	KindConfig       string
	MaxStartAttempts int
	Status           ClusterStatus
	Registry         *ClusterRegistry
	KubeClient       *kubernetes.Clientset
	RegistryPort     string
	KubeResourcer
}

type KubeClusterOption func(kc *KubeCluster)

func WithName(name string) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.Name = name
	}
}

func WithKubeConfigPath(path string) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.KubeConfigPath = path
	}
}

func WithKindConfig(config string) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.KindConfig = config
	}
}

func ShouldStartOnCreation(start bool) KubeClusterOption {
	return func(kc *KubeCluster) {
		if start {
			kc.Status = Dead
		} else {
			kc.Status = Alive
		}
	}
}

func WithMaxAttempts(attempts int) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.MaxStartAttempts = attempts
	}
}

func WithRegistry(registry *ClusterRegistry, port string) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.Registry = registry
		kc.RegistryPort = port
	}
}

func WithRegistryPort(port string) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.RegistryPort = port
	}
}

func WithKubeClient(kubeclient *kubernetes.Clientset) KubeClusterOption {
	return func(kc *KubeCluster) {
		kc.KubeClient = kubeclient
	}
}

func NewKubeCluster(options ...KubeClusterOption) (*KubeCluster, error) {
	provider := NewProvider()
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("NewKubeCluster: %w", err)
	}

	c := &KubeCluster{
		Provider:         provider,
		Name:             "",
		KubeConfigPath:   filepath.Join(home, ".kube", "kind-config.yaml"),
		KindConfig:       "",
		Status:           Dead,
		MaxStartAttempts: 5,
		RegistryPort:     "5000",
		KubeResourcer:    nil,
	}

	for _, option := range options {
		option(c)
	}

	if c.Status == Dead {
		err = c.Start()
		if err != nil {
			return nil, fmt.Errorf("NewKubeCluster: %w", err)
		}
	}

	if c.Registry == nil {
		registry, err := NewRegistry(context.TODO(), c.RegistryPort, c.RegistryPort)
		if err != nil {
			return nil, fmt.Errorf("NewKubeCluster: %w", err)
		}

		c.Registry = registry
	}

	if c.KubeClient == nil {
		kubeclient, err := createKubeClient(c.KubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("NewKubeCluster: %w", err)
		}

		c.KubeClient = kubeclient
	}

	if c.KubeResourcer == nil {
		resourcer, err := NewKubeResourceManager(c.KubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("NewKubeCluster: %w", err)
		}

		c.KubeResourcer = resourcer
	}

	return c, nil
}

//create creates the kind cluster. It will retry up to maxAttempts times
func (kc *KubeCluster) Start() error {
	if kc.Status == Alive {
		return nil
	}

	exists, err := checkForExistingCluster(kc.Provider, kc.Name)
	if err != nil {
		return fmt.Errorf("KubeCluster.start: %w", err)
	}

	if exists {
		return fmt.Errorf("KubeCluster.start: %w", &ExistingKubeClusterError{
			name: kc.Name,
		})
	}

	err = createKubeConfig(kc.KubeConfigPath, kc.Name)
	if err != nil {
		return fmt.Errorf("KubeCluster.start: %w", err)
	}

	for attempts := 0; attempts < kc.MaxStartAttempts; attempts++ {
		err := kc.Provider.Create(
			kc.Name,
			cluster.CreateWithNodeImage(""),
			cluster.CreateWithRetain(false),
			cluster.CreateWithWaitForReady(time.Duration(0)),
			cluster.CreateWithKubeconfigPath(kc.KubeConfigPath),
			cluster.CreateWithDisplayUsage(false),
			cluster.CreateWithRawConfig([]byte(kc.KindConfig)),
		)
		if err != nil {
			if attempts == kc.MaxStartAttempts-1 {
				return fmt.Errorf("KubeCluster.start: %w", &ExceededMaxAttemptError{
					attempts: kc.MaxStartAttempts,
				})
			}

			fmt.Printf("Error bringing up cluster, will retry (attempt %d): %v\n", attempts+1, err)
			continue
		}

		break
	}

	kc.Status = Alive
	return nil
}

func (kc *KubeCluster) Delete() error {
	if kc.Status == Dead {
		return nil
	}

	exists, err := checkForExistingCluster(kc.Provider, kc.Name)
	if err != nil {
		return fmt.Errorf("Delete: %s", err)
	}

	if exists {
		err := kc.Provider.Delete(kc.Name, kc.KubeConfigPath)
		if err != nil {
			return fmt.Errorf("KubeCluster.Delete: %s", err)
		}
	}

	exists, err = checkKubeConfig(kc.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("KubeCluster.Delete: %s", err)
	}

	if exists {
		err = os.Remove(kc.KubeConfigPath)
		if err != nil {
			return fmt.Errorf("KubeCluster.Delete: %s", err)
		}
	}

	err = kc.Registry.Delete(context.TODO())
	if err != nil {
		return fmt.Errorf("KubeCluster.Delete: %s", err)
	}

	return nil
}

func createKubeConfig(path string, clusterName string) error {
	fmt.Println("Creating kubeconfig...")

	exists, err := checkKubeConfig(path)
	if err != nil {
		return fmt.Errorf("createKubeConfig: %w", err)
	}

	if exists {
		return fmt.Errorf("createKubeConfig: %w", &ExistingKubeConfigError{
			path: path,
		})
	}

	err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return fmt.Errorf("createKubeConfig: %w", err)
	}

	_, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("createKubeConfig: %w", err)
	}

	return nil
}

func checkKubeConfig(path string) (bool, error) {
	_, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("checkKubeConfig: %w", err)
	}

	return true, nil
}

func checkForExistingCluster(provider *cluster.Provider, clusterName string) (bool, error) {
	nodes, err := provider.ListNodes(clusterName)
	if err != nil {
		return false, fmt.Errorf("checkForExistingCluster: %w", err)
	}

	if len(nodes) != 0 {
		return true, nil
	}

	return false, nil
}

func createKubeClient(path string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, fmt.Errorf("createKubeClient: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("createKubeClient: %s", err)
	}

	return clientset, nil
}

func NewProvider() *cluster.Provider {
	return cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
}
