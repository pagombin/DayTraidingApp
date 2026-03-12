package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// container represents a subset of Docker container inspect response.
type container struct {
	ID    string            `json:"Id"`
	Names []string          `json:"Names"`
	Labels map[string]string `json:"Labels"`
}

// newDockerClient returns an *http.Client that communicates over the Docker
// Engine unix socket. This avoids pulling in the Docker SDK as a dependency.
func newDockerClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
			},
		},
	}
}

// RestartContainer finds a Docker container whose compose service label
// matches serviceName and restarts it via the Docker Engine API.
func RestartContainer(ctx context.Context, serviceName string) error {
	client := newDockerClient()

	containerID, err := findContainerByService(ctx, client, serviceName)
	if err != nil {
		return fmt.Errorf("find container for service %s: %w", serviceName, err)
	}

	// POST /containers/{id}/restart with a 30-second timeout.
	url := fmt.Sprintf("http://localhost/containers/%s/restart?t=30", containerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create restart request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("restart request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restart failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// findContainerByService lists all containers and returns the ID of the one
// whose com.docker.compose.service label matches the given serviceName.
func findContainerByService(ctx context.Context, client *http.Client, serviceName string) (string, error) {
	url := "http://localhost/containers/json?all=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create list request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("list containers request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("list containers failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var containers []container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return "", fmt.Errorf("decode container list: %w", err)
	}

	for _, c := range containers {
		if label, ok := c.Labels["com.docker.compose.service"]; ok && label == serviceName {
			return c.ID, nil
		}
	}

	return "", fmt.Errorf("no container found with compose service label %q", serviceName)
}
