package sabnzbd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
)

// Client provides access to the SABnzbd API.
type Client struct {
	baseURL string
	apiKey  string
}

// New creates a SABnzbd client using config and discovered keys.
func New() *Client {
	return &Client{
		baseURL: config.ServiceURL("sabnzbd"),
		apiKey:  keys.Get("sabnzbd"),
	}
}

// QueueSlot represents a single download in the SABnzbd queue.
type QueueSlot struct {
	Filename   string  `json:"filename"`
	Status     string  `json:"status"`
	Percentage string  `json:"percentage"`
	SizeLeft   string  `json:"sizeleft"`
	Size       string  `json:"size"`
	MBLeft     float64 `json:"mbleft"`
	MB         float64 `json:"mb"`
}

// QueueResponse represents the SABnzbd queue API response.
type QueueResponse struct {
	Speed      string      `json:"speed"`
	SizeLeft   string      `json:"sizeleft"`
	NoOfSlots  int         `json:"noofslots_total"`
	Slots      []QueueSlot `json:"slots"`
	Status     string      `json:"status"`
	Paused     bool        `json:"paused"`
	SpeedLimit string      `json:"speedlimit"`
}

// Queue fetches the current SABnzbd download queue.
func (c *Client) Queue() (*QueueResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("sabnzbd API key not found")
	}

	url := fmt.Sprintf("%s/api?mode=queue&output=json&apikey=%s", c.baseURL, c.apiKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var wrapper struct {
		Queue QueueResponse `json:"queue"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Queue, nil
}

// Version returns the SABnzbd version string.
func (c *Client) Version() (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("sabnzbd API key not found")
	}

	url := fmt.Sprintf("%s/api?mode=version&output=json&apikey=%s", c.baseURL, c.apiKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	var result struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Version, nil
}
