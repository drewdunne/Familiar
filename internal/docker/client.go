package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

// Client wraps the Docker client with convenience methods.
type Client struct {
	cli *client.Client
}

// NewClient creates a new Docker client.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the Docker client.
func (c *Client) Close() error {
	return c.cli.Close()
}

// Ping checks if Docker daemon is accessible.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// ImageExists checks if an image exists locally.
func (c *Client) ImageExists(ctx context.Context, imageName string) (bool, error) {
	images, err := c.cli.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", imageName)),
	})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

// PullImage pulls an image if it doesn't exist.
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	exists, err := c.ImageExists(ctx, imageName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}
	defer reader.Close()

	// Consume the output
	_, err = io.Copy(io.Discard, reader)
	return err
}

// ContainerConfig holds configuration for creating a container.
type ContainerConfig struct {
	Name        string
	Image       string
	User        string // Container user (e.g. "1000" for UID)
	WorkDir     string
	Mounts      []Mount
	TmpfsMounts []TmpfsMount // In-memory filesystem mounts
	Env         []string
	Labels      map[string]string
	Cmd         []string
	Entrypoint  []string
	NetworkMode string // e.g. "host", "bridge", or empty for default
}

// Mount represents a bind mount.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// TmpfsMount represents a tmpfs mount (in-memory filesystem).
type TmpfsMount struct {
	Target string
	Mode   uint32 // File mode (e.g., 0777 for world-writable)
}

// CreateContainer creates a new container.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	mounts := make([]mount.Mount, 0, len(cfg.Mounts)+len(cfg.TmpfsMounts))

	// Add bind mounts
	for _, m := range cfg.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.Source,
			Target:   m.Target,
			ReadOnly: m.ReadOnly,
		})
	}

	// Add tmpfs mounts
	for _, m := range cfg.TmpfsMounts {
		tmpfsMount := mount.Mount{
			Type:   mount.TypeTmpfs,
			Target: m.Target,
		}
		if m.Mode != 0 {
			tmpfsMount.TmpfsOptions = &mount.TmpfsOptions{
				Mode: os.FileMode(m.Mode),
			}
		}
		mounts = append(mounts, tmpfsMount)
	}

	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}
	if cfg.NetworkMode != "" {
		hostConfig.NetworkMode = container.NetworkMode(cfg.NetworkMode)
	}

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:      cfg.Image,
			User:       cfg.User,
			WorkingDir: cfg.WorkDir,
			Env:        cfg.Env,
			Labels:     cfg.Labels,
			Cmd:        cfg.Cmd,
			Entrypoint: cfg.Entrypoint,
			Tty:        true,
			OpenStdin:  true,
		},
		hostConfig,
		nil, nil, cfg.Name,
	)
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// StopContainer stops a container.
func (c *Client) StopContainer(ctx context.Context, id string, timeout int) error {
	t := timeout
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

// ExecInContainer runs a command in a container and waits for it to complete.
// It returns an error if the command exits with a non-zero exit code.
func (c *Client) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	execConfig, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd: cmd,
	})
	if err != nil {
		return fmt.Errorf("creating exec: %w", err)
	}

	if err := c.cli.ContainerExecStart(ctx, execConfig.ID, container.ExecStartOptions{}); err != nil {
		return fmt.Errorf("starting exec: %w", err)
	}

	// Wait for the exec to complete and check exit code
	inspect, err := c.cli.ContainerExecInspect(ctx, execConfig.ID)
	if err != nil {
		return fmt.Errorf("inspecting exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", inspect.ExitCode)
	}

	return nil
}

// ContainerInspect holds selected fields from a container inspection.
type ContainerInspect struct {
	Mounts []MountPoint
}

// MountPoint describes a mount in a running container.
type MountPoint struct {
	Source      string
	Destination string
	RO          bool
}

// InspectContainer returns inspection details for a container.
func (c *Client) InspectContainer(ctx context.Context, containerID string) (*ContainerInspect, error) {
	resp, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspecting container: %w", err)
	}

	mounts := make([]MountPoint, len(resp.Mounts))
	for i, m := range resp.Mounts {
		mounts[i] = MountPoint{
			Source:      m.Source,
			Destination: m.Destination,
			RO:          !m.RW,
		}
	}

	return &ContainerInspect{Mounts: mounts}, nil
}

// GetContainerLogs returns container logs.
func (c *Client) GetContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
	})
}
