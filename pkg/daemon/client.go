package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/urmzd/zigbee-skill/pkg/device"
)

// DaemonClient implements device.Controller by proxying to the daemon over a Unix socket.
type DaemonClient struct {
	http       *http.Client
	socketPath string
}

// NewClient creates a DaemonClient that talks to the daemon over the given Unix socket.
func NewClient(socketPath string) *DaemonClient {
	return &DaemonClient{
		socketPath: socketPath,
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		},
	}
}

func (c *DaemonClient) post(ctx context.Context, path string, reqBody any) (*http.Response, error) {
	var body bytes.Buffer
	if reqBody != nil {
		if err := json.NewEncoder(&body).Encode(reqBody); err != nil {
			return nil, err
		}
	} else {
		body.WriteString("{}")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://daemon"+path, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

func (c *DaemonClient) ListDevices(ctx context.Context) ([]device.Device, error) {
	resp, err := c.post(ctx, "/devices/list", nil)
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkErr(resp); err != nil {
		return nil, err
	}
	var result struct {
		Devices []device.Device `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Devices, nil
}

func (c *DaemonClient) GetDevice(ctx context.Context, id string) (*device.Device, error) {
	resp, err := c.post(ctx, "/devices/get", idRequest{ID: id})
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkErr(resp); err != nil {
		return nil, err
	}
	var result struct {
		Device device.Device `json:"device"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Device, nil
}

func (c *DaemonClient) RenameDevice(ctx context.Context, id, newName string) error {
	resp, err := c.post(ctx, "/devices/rename", renameRequest{ID: id, Name: newName})
	if err != nil {
		return fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return checkErr(resp)
}

func (c *DaemonClient) RemoveDevice(ctx context.Context, id string, force bool) error {
	resp, err := c.post(ctx, "/devices/remove", removeRequest{ID: id, Force: force})
	if err != nil {
		return fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return checkErr(resp)
}

func (c *DaemonClient) ClearDevices(ctx context.Context) error {
	resp, err := c.post(ctx, "/devices/clear", struct{}{})
	if err != nil {
		return fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return checkErr(resp)
}

func (c *DaemonClient) GetDeviceState(ctx context.Context, id string) (device.DeviceState, error) {
	resp, err := c.post(ctx, "/devices/state", idRequest{ID: id})
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkErr(resp); err != nil {
		return nil, err
	}
	var result struct {
		State device.DeviceState `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.State, nil
}

func (c *DaemonClient) SetDeviceState(ctx context.Context, id string, state map[string]any) (device.DeviceState, error) {
	resp, err := c.post(ctx, "/devices/set", setStateRequest{ID: id, State: state})
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkErr(resp); err != nil {
		return nil, err
	}
	var result struct {
		State device.DeviceState `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.State, nil
}

func (c *DaemonClient) PermitJoin(ctx context.Context, enable bool, duration int) error {
	resp, err := c.post(ctx, "/discovery/permit", permitRequest{Enable: enable, Duration: duration})
	if err != nil {
		return fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return checkErr(resp)
}

func (c *DaemonClient) IsConnected() bool {
	resp, err := c.post(context.Background(), "/health", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var result struct {
		Connected bool `json:"connected"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Connected
}

func (c *DaemonClient) Close() {}

// checkErr reads an error response from the daemon and maps it to sentinel errors.
func checkErr(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var result struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	msg := result.Error
	if msg == "" {
		msg = resp.Status
	}

	switch {
	case resp.StatusCode == http.StatusNotFound || strings.Contains(msg, "not found"):
		return device.ErrNotFound
	case resp.StatusCode == http.StatusServiceUnavailable || strings.Contains(msg, "not connected"):
		return device.ErrNotConnected
	case resp.StatusCode == http.StatusBadRequest && strings.Contains(msg, "validation"):
		return fmt.Errorf("%w: %s", device.ErrValidation, msg)
	case resp.StatusCode == http.StatusGatewayTimeout || strings.Contains(msg, "timed out"):
		return fmt.Errorf("%w: %s", device.ErrTimeout, msg)
	default:
		return fmt.Errorf("daemon error: %s", msg)
	}
}

// DaemonEventSubscriber streams discovery events from the daemon over SSE.
type DaemonEventSubscriber struct {
	socketPath string
	cancel     context.CancelFunc
}

// NewDaemonEventSubscriber creates an event subscriber connected to the daemon.
func NewDaemonEventSubscriber(socketPath string) *DaemonEventSubscriber {
	return &DaemonEventSubscriber{socketPath: socketPath}
}

func (s *DaemonEventSubscriber) Subscribe() chan device.DiscoveryEvent {
	ch := make(chan device.DiscoveryEvent, 16)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		defer close(ch)

		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", s.socketPath)
				},
			},
		}
		req, err := http.NewRequestWithContext(ctx, "GET", "http://daemon/discovery/events", nil)
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var ev device.DiscoveryEvent
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

func (s *DaemonEventSubscriber) Unsubscribe(ch chan device.DiscoveryEvent) {
	if s.cancel != nil {
		s.cancel()
	}
}
