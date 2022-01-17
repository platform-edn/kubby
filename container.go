package kubby

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Container struct {
	Client   *client.Client
	Id       string
	Name     string
	Image    string
	Tag      string
	Networks []string
	Ports    map[string]string
}

type ContainerOption func(c *Container)

func WithContainerName(name string) ContainerOption {
	return func(c *Container) {
		c.Name = name
	}
}

func WithImage(image string) ContainerOption {
	return func(c *Container) {
		c.Image = image
	}
}

func WithTag(tag string) ContainerOption {
	return func(c *Container) {
		c.Tag = tag
	}
}

func WithNetwork(network string) ContainerOption {
	return func(c *Container) {
		c.Networks = append(c.Networks, network)
	}
}

func WithPort(imagePort string, hostPort string) ContainerOption {
	return func(c *Container) {
		c.Ports[imagePort] = hostPort
	}
}

func WithClient(cli *client.Client) ContainerOption {
	return func(c *Container) {
		c.Client = cli
	}
}

func NewContainer(ctx context.Context, options ...ContainerOption) (*Container, error) {
	c := &Container{
		Tag: "latest",
	}

	for _, option := range options {
		option(c)
	}

	if c.Name == "" {
		return nil, fmt.Errorf("NewContainer: %w", &MissingFieldError{
			field: "Name",
		})
	}
	if c.Image == "" {
		return nil, fmt.Errorf("NewContainer: %w", &MissingFieldError{
			field: "Image",
		})
	}
	if c.Client == nil {
		cli, err := NewContainerClient()
		if err != nil {
			return nil, fmt.Errorf("NewContainer: %w", err)
		}

		c.Client = cli
	}

	err := c.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewContainer: %w", err)
	}

	return c, nil
}

func (c *Container) Start(ctx context.Context) error {
	fullImage := fmt.Sprintf("%s:%s", c.Image, c.Tag)
	err := pullImage(ctx, c.Client, fullImage)
	if err != nil {
		return fmt.Errorf("Container.Start: %w", err)
	}

	portMap, err := portsConfig(c.Ports)
	if err != nil {
		return fmt.Errorf("Container.Start: %w", err)
	}

	endpoints := endpointConfig(c.Networks, c.Name)

	cont, err := c.Client.ContainerCreate(
		ctx,
		&container.Config{
			Image: fullImage,
		},
		&container.HostConfig{
			PortBindings: portMap,
		},
		&network.NetworkingConfig{
			EndpointsConfig: endpoints,
		},
		nil,
		c.Name,
	)
	if err != nil {
		return fmt.Errorf("Container.Start: %w", err)
	}

	c.Id = cont.ID

	err = c.Client.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("Container.Start: %w", err)
	}

	return nil
}

func (c *Container) Stop(ctx context.Context) error {
	timeout := time.Second * 5
	err := c.Client.ContainerStop(ctx, c.Id, &timeout)
	if err != nil {
		return fmt.Errorf("Container.Stop: %w", err)
	}

	return nil
}

func (c *Container) Delete(ctx context.Context) error {
	err := c.Stop(ctx)
	if err != nil {
		return fmt.Errorf("Container.Delete: %w", err)
	}

	err = c.Client.ContainerRemove(ctx, c.Id, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Container.Delete: %w", err)
	}

	return nil
}

func pullImage(ctx context.Context, cli *client.Client, image string) error {
	fmt.Printf("pulling %s...\n", image)
	out, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pullImage: %w", err)
	}

	_, err = io.Copy(ioutil.Discard, out)
	if err != nil {
		return fmt.Errorf("pullImage: %w", err)
	}
	out.Close()

	return nil
}

func portsConfig(ports map[string]string) (nat.PortMap, error) {
	portMap := nat.PortMap{}

	for imagePort, hostPort := range ports {
		hostBinding := nat.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: hostPort,
		}

		containerBinding, err := nat.NewPort("tcp", imagePort)
		if err != nil {
			return nil, fmt.Errorf("portsConfig: %w", err)
		}

		portMap[containerBinding] = []nat.PortBinding{hostBinding}
	}

	return portMap, nil

}

func endpointConfig(networks []string, containerName string) map[string]*network.EndpointSettings {
	endpoints := map[string]*network.EndpointSettings{}

	for _, nw := range networks {
		point := &network.EndpointSettings{
			Aliases: []string{containerName},
		}

		endpoints[nw] = point
	}

	return endpoints
}

func GetContainerId(name string) (string, error) {
	cli, err := NewContainerClient()
	if err != nil {
		return "", fmt.Errorf("GetContainerId: %w", err)
	}

	containers, err := cli.ContainerList(context.TODO(), types.ContainerListOptions{})
	if err != nil {
		return "", fmt.Errorf("GetContainerId: %w", err)
	}

	for _, container := range containers {
		for _, n := range container.Names {
			if name == strings.Trim(n, "/") {
				return n, nil
			}
		}
	}

	return "", fmt.Errorf("GetContainerId: %w", &BadContainerNameError{
		name: name,
	})
}

func NewContainerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return nil, fmt.Errorf("NewContainerClient: %w", err)
	}

	return cli, nil
}
