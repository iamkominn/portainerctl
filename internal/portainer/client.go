package portainer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"portainerctl/internal/model"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
	apiKey     string
}

type authRequest struct {
	Username string `json:"Username"`
	Password string `json:"Password"`
}

type authResponse struct {
	JWT string `json:"jwt"`
}

func NewClient(rawURL string, insecure bool) (*Client, error) {
	baseURL, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}, nil
}

func (c *Client) Login(ctx context.Context, username, password string) error {
	reqBody := authRequest{Username: username, Password: password}
	var resp authResponse
	if err := c.do(ctx, http.MethodPost, "/api/auth", nil, reqBody, &resp); err != nil {
		return err
	}

	if resp.JWT == "" {
		return fmt.Errorf("authentication succeeded but no JWT token was returned")
	}

	c.token = resp.JWT
	c.apiKey = ""
	return nil
}

func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = strings.TrimSpace(apiKey)
	if c.apiKey != "" {
		c.token = ""
	}
}

func (c *Client) ListEnvironments(ctx context.Context) ([]model.Environment, error) {
	var envs []model.Environment
	if err := c.do(ctx, http.MethodGet, "/api/endpoints", nil, nil, &envs); err != nil {
		return nil, err
	}
	sort.Slice(envs, func(i, j int) bool { return strings.ToLower(envs[i].Name) < strings.ToLower(envs[j].Name) })
	return envs, nil
}

func (c *Client) ListContainers(ctx context.Context, endpointID int) ([]model.Container, error) {
	query := url.Values{}
	query.Set("all", "1")

	var containers []model.Container
	apiPath := fmt.Sprintf("/api/endpoints/%d/docker/containers/json", endpointID)
	if err := c.do(ctx, http.MethodGet, apiPath, query, nil, &containers); err != nil {
		return nil, err
	}

	sort.Slice(containers, func(i, j int) bool { return containerName(containers[i]) < containerName(containers[j]) })
	return containers, nil
}

func (c *Client) StartContainer(ctx context.Context, endpointID int, containerID string) error {
	return c.doNoContent(ctx, http.MethodPost, fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/start", endpointID, containerID), nil)
}

func (c *Client) StopContainer(ctx context.Context, endpointID int, containerID string) error {
	return c.doNoContent(ctx, http.MethodPost, fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/stop", endpointID, containerID), nil)
}

func (c *Client) RestartContainer(ctx context.Context, endpointID int, containerID string) error {
	return c.doNoContent(ctx, http.MethodPost, fmt.Sprintf("/api/endpoints/%d/docker/containers/%s/restart", endpointID, containerID), nil)
}

func (c *Client) RemoveContainer(ctx context.Context, endpointID int, containerID string) error {
	query := url.Values{}
	query.Set("force", "1")
	query.Set("v", "1")
	return c.doNoContent(ctx, http.MethodDelete, fmt.Sprintf("/api/endpoints/%d/docker/containers/%s", endpointID, containerID), query)
}

func (c *Client) ListStacks(ctx context.Context, endpointID int) ([]model.Stack, error) {
	var stacks []model.Stack
	if err := c.do(ctx, http.MethodGet, "/api/stacks", nil, nil, &stacks); err != nil {
		return nil, err
	}

	filtered := make([]model.Stack, 0, len(stacks))
	for _, stack := range stacks {
		if stack.EndpointID == endpointID {
			stack.Origin = "Full"
			filtered = append(filtered, stack)
		}
	}

	externalStacks, err := c.listExternalComposeStacks(ctx, endpointID)
	if err != nil {
		return nil, err
	}

	known := make(map[string]struct{}, len(filtered))
	for _, stack := range filtered {
		known[strings.ToLower(stack.Name)] = struct{}{}
	}
	for _, stack := range externalStacks {
		if _, exists := known[strings.ToLower(stack.Name)]; exists {
			continue
		}
		filtered = append(filtered, stack)
	}

	sort.Slice(filtered, func(i, j int) bool { return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name) })
	return filtered, nil
}

func (c *Client) StartStack(ctx context.Context, stackID, endpointID int) error {
	query := url.Values{}
	query.Set("endpointId", strconv.Itoa(endpointID))
	return c.doNoContent(ctx, http.MethodPost, fmt.Sprintf("/api/stacks/%d/start", stackID), query)
}

func (c *Client) StopStack(ctx context.Context, stackID, endpointID int) error {
	query := url.Values{}
	query.Set("endpointId", strconv.Itoa(endpointID))
	return c.doNoContent(ctx, http.MethodPost, fmt.Sprintf("/api/stacks/%d/stop", stackID), query)
}

func (c *Client) RemoveStack(ctx context.Context, stackID, endpointID int) error {
	query := url.Values{}
	query.Set("endpointId", strconv.Itoa(endpointID))
	return c.doNoContent(ctx, http.MethodDelete, fmt.Sprintf("/api/stacks/%d", stackID), query)
}

func (c *Client) ListImages(ctx context.Context, endpointID int) ([]model.Image, error) {
	var images []model.Image
	apiPath := fmt.Sprintf("/api/endpoints/%d/docker/images/json", endpointID)
	if err := c.do(ctx, http.MethodGet, apiPath, nil, nil, &images); err != nil {
		return nil, err
	}
	sort.Slice(images, func(i, j int) bool { return imageName(images[i]) < imageName(images[j]) })
	return images, nil
}

func (c *Client) RemoveImage(ctx context.Context, endpointID int, imageID string) error {
	query := url.Values{}
	query.Set("force", "1")
	query.Set("noprune", "0")
	return c.doNoContent(ctx, http.MethodDelete, fmt.Sprintf("/api/endpoints/%d/docker/images/%s", endpointID, imageID), query)
}

func (c *Client) listExternalComposeStacks(ctx context.Context, endpointID int) ([]model.Stack, error) {
	containers, err := c.ListContainers(ctx, endpointID)
	if err != nil {
		return nil, err
	}

	byProject := map[string][]model.Container{}
	for _, container := range containers {
		project := strings.TrimSpace(container.Labels["com.docker.compose.project"])
		if project == "" {
			continue
		}
		byProject[project] = append(byProject[project], container)
	}

	stacks := make([]model.Stack, 0, len(byProject))
	for project, grouped := range byProject {
		stack := model.Stack{
			ID:      0,
			Name:    project,
			Type:    2,
			Origin:  "Limited",
			Limited: true,
		}

		if len(grouped) > 0 {
			stack.CreationDate = grouped[0].Created
			stack.UpdateDate = grouped[0].Created
		}

		active := false
		for _, container := range grouped {
			if stack.CreationDate == 0 || (container.Created > 0 && container.Created < stack.CreationDate) {
				stack.CreationDate = container.Created
			}
			if container.Created > stack.UpdateDate {
				stack.UpdateDate = container.Created
			}
			if strings.EqualFold(container.State, "running") {
				active = true
			}
		}

		if active {
			stack.Status = 1
		} else {
			stack.Status = 2
		}

		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func (c *Client) doNoContent(ctx context.Context, method, apiPath string, query url.Values) error {
	return c.do(ctx, method, apiPath, query, nil, nil)
}

func (c *Client) do(ctx context.Context, method, apiPath string, query url.Values, requestBody any, out any) error {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, apiPath)
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var body io.Reader
	if requestBody != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(requestBody); err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		msg := strings.TrimSpace(string(bodyBytes))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("api %s %s: %s", method, apiPath, msg)
	}

	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func containerName(c model.Container) string {
	if len(c.Names) == 0 {
		return c.ID
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

func imageName(img model.Image) string {
	if len(img.RepoTags) == 0 {
		return img.ID
	}
	return img.RepoTags[0]
}
