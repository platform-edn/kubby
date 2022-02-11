package kubby

import "fmt"

type NodePort struct {
	Host      string
	Container string
}

type KindConfig struct {
	Name              string
	ControlPlaneNodes int
	WorkerNodes       int
	NodePorts         []*NodePort
	RegistryAddress   string
	RegistryPort      string
}

func NewKindConfig(name string, cnCount int, wnCount int, np []*NodePort, ra string, rp string) *KindConfig {
	config := &KindConfig{
		Name:              name,
		ControlPlaneNodes: cnCount,
		WorkerNodes:       wnCount,
		NodePorts:         np,
		RegistryAddress:   ra,
		RegistryPort:      rp,
	}

	return config
}

var (
	kindHeaderFormat = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: %s
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%s"]
    endpoint = ["http://%s:%s"]
nodes:`
	portMappingFormat = `
  - containerPort: %s
    hostPort: %s`
)

func (config *KindConfig) String() string {
	kindConfig := fmt.Sprintf(kindHeaderFormat, config.Name, config.RegistryPort, config.RegistryAddress, config.RegistryPort)

	kindConfig = kindConfig + `
- role: control-plane
  extraPortMappings:`

	for _, port := range config.NodePorts {
		kindConfig = kindConfig + fmt.Sprintf(portMappingFormat, port.Container, port.Host)
	}

	for i := 1; i < config.ControlPlaneNodes; i++ {
		kindConfig = kindConfig + `
- role: control-plane`
	}

	for i := 0; i < config.WorkerNodes; i++ {
		kindConfig = kindConfig + `
- role: worker`
	}

	return kindConfig
}
