package kubby

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

type ClusterRegistry struct {
	Container
	Url string
}

func NewRegistry(ctx context.Context, hostPort string, imagePort string) (*ClusterRegistry, error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return nil, fmt.Errorf("NewRegistry: %w", err)
	}

	r := ClusterRegistry{
		Container: Container{
			Client:   cli,
			Name:     "kind-registry",
			Image:    "registry",
			Tag:      "2",
			Networks: []string{"kind"},
			Ports: map[string]string{
				imagePort: hostPort,
			},
		},
		Url: fmt.Sprintf("localhost:%s", hostPort),
	}

	err = r.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewRegistry: %w", err)
	}

	return &r, nil
}

func (r *ClusterRegistry) PushImage(ctx context.Context, dockerPath string, name string) error {
	image := fmt.Sprintf("%s/%s", r.Url, name)

	err := buildImage(ctx, r.Client, dockerPath, image)
	if err != nil {
		return fmt.Errorf("ClusterRegistry.PushImage: %w", err)
	}

	err = pushImage(ctx, r.Client, image)
	if err != nil {
		return fmt.Errorf("ClusterRegistry.PushImage: %w", err)
	}
	return nil
}

func buildImage(ctx context.Context, cli *client.Client, path string, image string) error {
	fmt.Printf("building %s...\n", image)
	tar, err := archive.TarWithOptions(path, &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("BuildImage: %w", err)
	}

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{image},
		Remove:     true,
	}

	res, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return fmt.Errorf("BuildImage: %w", err)
	}

	defer res.Body.Close()

	err = getDockerOutput(res.Body)
	if err != nil {
		return fmt.Errorf("BuildImage: %w", err)
	}

	return nil
}

func pushImage(ctx context.Context, cli *client.Client, image string) error {
	fmt.Printf("pushing %s...\n", image)
	res, err := cli.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: "holder",
	})
	if err != nil {
		return fmt.Errorf("pushImage: %w", err)
	}

	defer res.Close()

	err = getDockerOutput(res)
	if err != nil {
		return fmt.Errorf("pushImage: %w", err)
	}

	return nil
}

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func getDockerOutput(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return fmt.Errorf("CheckImageBuildOutput: %w", &BadImageBuildError{
			output: errLine.Error,
		})
	}

	err := scanner.Err()
	if err != nil {
		return fmt.Errorf("CheckImageBuildOutput: %w", err)
	}

	return nil
}
