package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// RESTEngineAdapter implements EngineClient over Docker/Podman Engine REST API.
type RESTEngineAdapter struct {
	client  *http.Client
	info    *EngineInfo
	baseURL string
}

func (a *RESTEngineAdapter) GetInfo() *EngineInfo {
	return a.info
}

func (a *RESTEngineAdapter) Ping(ctx context.Context) (*EngineInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/_ping", nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engine ping failed at socket %s: %w", a.info.SocketPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("engine ping returned non-200 status: %d", resp.StatusCode)
	}

	// Fetch version info
	vReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/version", nil)
	if vResp, vErr := a.client.Do(vReq); vErr == nil {
		defer vResp.Body.Close()
		var vData struct {
			Version    string `json:"Version"`
			APIVersion string `json:"ApiVersion"`
			OSType     string `json:"Os"`
		}
		if json.NewDecoder(vResp.Body).Decode(&vData) == nil {
			a.info.APIVersion = vData.APIVersion
			if vData.OSType != "" {
				a.info.OSType = vData.OSType
			}
		}
	}

	return a.info, nil
}

func (a *RESTEngineAdapter) ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error) {
	endpoint := fmt.Sprintf("%s/containers/json?all=%t", a.baseURL, all)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list containers failed (%d): %s", resp.StatusCode, string(body))
	}

	var containers []ContainerSummary
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode containers list: %w", err)
	}
	return containers, nil
}

func (a *RESTEngineAdapter) InspectContainer(ctx context.Context, nameOrID string) (*ContainerInspect, error) {
	endpoint := fmt.Sprintf("%s/containers/%s/json", a.baseURL, url.PathEscape(nameOrID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inspect container failed (%d): %s", resp.StatusCode, string(body))
	}

	var inspect ContainerInspect
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return nil, fmt.Errorf("failed to decode container inspection: %w", err)
	}
	return &inspect, nil
}

func (a *RESTEngineAdapter) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	endpoint := fmt.Sprintf("%s/containers/create?name=%s", a.baseURL, url.QueryEscape(spec.Name))

	// Prepare Docker/Podman compatible ContainerCreate payload
	type portBindingHost struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}

	exposedPorts := make(map[string]struct{})
	portBindings := make(map[string][]portBindingHost)

	for _, p := range spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		portKey := fmt.Sprintf("%d/%s", p.ContainerPort, proto)
		exposedPorts[portKey] = struct{}{}
		portBindings[portKey] = []portBindingHost{
			{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(p.HostPort),
			},
		}
	}

	var binds []string
	for _, v := range spec.Volumes {
		mode := v.Mode
		if mode == "" {
			if v.ReadOnly {
				mode = "ro"
			} else {
				mode = "rw"
			}
		}
		binds = append(binds, fmt.Sprintf("%s:%s:%s", v.Source, v.Target, mode))
	}

	restartPolicyName := "on-failure"
	restartMaxRetries := 5
	if spec.RestartPolicy == "always" {
		restartPolicyName = "always"
		restartMaxRetries = 0
	} else if spec.RestartPolicy == "no" || spec.RestartPolicy == "none" {
		restartPolicyName = "no"
		restartMaxRetries = 0
	}

	payload := map[string]interface{}{
		"Image":        spec.Image,
		"ExposedPorts": exposedPorts,
		"Env":          spec.Env,
		"Labels":       spec.Labels,
		"HostConfig": map[string]interface{}{
			"Binds":        binds,
			"PortBindings": portBindings,
			"RestartPolicy": map[string]interface{}{
				"Name":              restartPolicyName,
				"MaximumRetryCount": restartMaxRetries,
			},
			"NetworkMode": spec.Network,
		},
	}

	if len(spec.Cmd) > 0 {
		payload["Cmd"] = spec.Cmd
	}
	if len(spec.Entrypoint) > 0 {
		payload["Entrypoint"] = spec.Entrypoint
	}
	if spec.User != "" {
		payload["User"] = spec.User
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create container failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("failed to parse create container response: %w", err)
	}

	return res.ID, nil
}

func (a *RESTEngineAdapter) StartContainer(ctx context.Context, nameOrID string) error {
	endpoint := fmt.Sprintf("%s/containers/%s/start", a.baseURL, url.PathEscape(nameOrID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("start container failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) StopContainer(ctx context.Context, nameOrID string, timeoutSeconds int) error {
	endpoint := fmt.Sprintf("%s/containers/%s/stop?t=%d", a.baseURL, url.PathEscape(nameOrID), timeoutSeconds)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop container failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) RemoveContainer(ctx context.Context, nameOrID string, force bool) error {
	endpoint := fmt.Sprintf("%s/containers/%s?force=%t", a.baseURL, url.PathEscape(nameOrID), force)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove container failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) RenameContainer(ctx context.Context, oldName, newName string) error {
	endpoint := fmt.Sprintf("%s/containers/%s/rename?name=%s", a.baseURL, url.PathEscape(oldName), url.QueryEscape(newName))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rename container failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) ListVolumes(ctx context.Context) ([]VolumeInspect, error) {
	endpoint := fmt.Sprintf("%s/volumes", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list volumes failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		Volumes []VolumeInspect `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Volumes, nil
}

func (a *RESTEngineAdapter) InspectVolume(ctx context.Context, name string) (*VolumeInspect, error) {
	endpoint := fmt.Sprintf("%s/volumes/%s", a.baseURL, url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inspect volume failed (%d): %s", resp.StatusCode, string(body))
	}

	var vol VolumeInspect
	if err := json.NewDecoder(resp.Body).Decode(&vol); err != nil {
		return nil, err
	}
	return &vol, nil
}

func (a *RESTEngineAdapter) CreateVolume(ctx context.Context, name string, labels map[string]string) (*VolumeInspect, error) {
	endpoint := fmt.Sprintf("%s/volumes/create", a.baseURL)
	payload := map[string]interface{}{
		"Name":   name,
		"Labels": labels,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create volume failed (%d): %s", resp.StatusCode, string(body))
	}

	var vol VolumeInspect
	if err := json.NewDecoder(resp.Body).Decode(&vol); err != nil {
		return nil, err
	}
	return &vol, nil
}

func (a *RESTEngineAdapter) RemoveVolume(ctx context.Context, name string, force bool) error {
	endpoint := fmt.Sprintf("%s/volumes/%s?force=%t", a.baseURL, url.PathEscape(name), force)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove volume failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) ListNetworks(ctx context.Context) ([]NetworkInspect, error) {
	endpoint := fmt.Sprintf("%s/networks", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list networks failed (%d): %s", resp.StatusCode, string(body))
	}

	var networks []NetworkInspect
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return nil, err
	}
	return networks, nil
}

func (a *RESTEngineAdapter) InspectNetwork(ctx context.Context, nameOrID string) (*NetworkInspect, error) {
	endpoint := fmt.Sprintf("%s/networks/%s", a.baseURL, url.PathEscape(nameOrID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inspect network failed (%d): %s", resp.StatusCode, string(body))
	}

	var net NetworkInspect
	if err := json.NewDecoder(resp.Body).Decode(&net); err != nil {
		return nil, err
	}
	return &net, nil
}

func (a *RESTEngineAdapter) CreateNetwork(ctx context.Context, name string, labels map[string]string) (*NetworkInspect, error) {
	endpoint := fmt.Sprintf("%s/networks/create", a.baseURL)
	payload := map[string]interface{}{
		"Name":           name,
		"CheckDuplicate": true,
		"Labels":         labels,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create network failed (%d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &NetworkInspect{
		ID:     res.ID,
		Name:   name,
		Labels: labels,
	}, nil
}

func (a *RESTEngineAdapter) RemoveNetwork(ctx context.Context, nameOrID string) error {
	endpoint := fmt.Sprintf("%s/networks/%s", a.baseURL, url.PathEscape(nameOrID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove network failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *RESTEngineAdapter) InspectImage(ctx context.Context, imageName string) (bool, error) {
	endpoint := fmt.Sprintf("%s/images/%s/json", a.baseURL, url.PathEscape(imageName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("inspect image failed (%d): %s", resp.StatusCode, string(body))
}
