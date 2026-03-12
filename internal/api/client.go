package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/maxtechera/admirarr/internal/config"
	"github.com/maxtechera/admirarr/internal/keys"
)

var client = &http.Client{Timeout: 5 * time.Second}

// Get performs an authenticated GET request to a service endpoint.
// Returns the raw body bytes and any error.
func Get(service, endpoint string, params map[string]string, timeout ...time.Duration) ([]byte, error) {
	t := 5 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	c := &http.Client{Timeout: t}
	u := buildURL(service, endpoint, params)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	addAuth(req, service)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// GetJSON performs an authenticated GET and unmarshals the JSON response.
func GetJSON(service, endpoint string, params map[string]string, target interface{}, timeout ...time.Duration) error {
	data, err := Get(service, endpoint, params, timeout...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Post performs an authenticated POST request with a JSON body.
func Post(service, endpoint string, body interface{}, params map[string]string, timeout ...time.Duration) ([]byte, error) {
	t := 10 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	c := &http.Client{Timeout: t}
	u := buildURL(service, endpoint, params)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequest("POST", u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	addAuth(req, service)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// PostJSON performs an authenticated POST and unmarshals the JSON response.
func PostJSON(service, endpoint string, body interface{}, params map[string]string, target interface{}, timeout ...time.Duration) error {
	data, err := Post(service, endpoint, body, params, timeout...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Put performs an authenticated PUT request with a JSON body.
func Put(service, endpoint string, body interface{}, params map[string]string, timeout ...time.Duration) ([]byte, error) {
	t := 10 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	c := &http.Client{Timeout: t}
	u := buildURL(service, endpoint, params)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequest("PUT", u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	addAuth(req, service)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// PutJSON performs an authenticated PUT and unmarshals the JSON response.
func PutJSON(service, endpoint string, body interface{}, params map[string]string, target interface{}, timeout ...time.Duration) error {
	data, err := Put(service, endpoint, body, params, timeout...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Delete performs an authenticated DELETE request.
func Delete(service, endpoint string, params map[string]string, timeout ...time.Duration) ([]byte, error) {
	t := 5 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	c := &http.Client{Timeout: t}
	u := buildURL(service, endpoint, params)

	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	addAuth(req, service)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CheckReachable checks if a service is reachable.
func CheckReachable(service string) bool {
	key := keys.Get(service)
	endpoints := map[string]string{
		"plex":         "identity",
		"qbittorrent":  "api/v2/app/version",
		"prowlarr":     fmt.Sprintf("api/v1/health?apikey=%s", key),
		"sonarr":       fmt.Sprintf("api/v3/health?apikey=%s", key),
		"radarr":       fmt.Sprintf("api/v3/health?apikey=%s", key),
		"tautulli":     fmt.Sprintf("api/v2?apikey=%s&cmd=status", key),
		"seerr":        "api/v1/status",
		"bazarr":       "",
		"organizr":     "",
		"flaresolverr": "",
	}

	ep, ok := endpoints[service]
	if !ok {
		ep = ""
	}

	u := fmt.Sprintf("%s/%s", config.ServiceURL(service), ep)
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(u)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func buildURL(service, endpoint string, params map[string]string) string {
	key := keys.Get(service)
	p := make(url.Values)

	// Add API key as query param for *Arr services
	if key != "" {
		switch service {
		case "sonarr", "radarr", "prowlarr":
			p.Set("apikey", key)
		case "plex":
			p.Set("X-Plex-Token", key)
		}
	}

	for k, v := range params {
		p.Set(k, v)
	}

	u := fmt.Sprintf("%s/%s", config.ServiceURL(service), endpoint)
	if encoded := p.Encode(); encoded != "" {
		u += "?" + encoded
	}
	return u
}

func addAuth(req *http.Request, service string) {
	key := keys.Get(service)
	if key == "" {
		return
	}
	switch service {
	case "seerr":
		req.Header.Set("X-Api-Key", key)
	}
}
