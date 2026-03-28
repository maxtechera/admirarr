package vpn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.mullvad.net"

// Account represents a Mullvad account.
type Account struct {
	Number    string    `json:"number"`
	ExpiresAt time.Time `json:"expiry"`
}

// AuthToken holds a Mullvad API access token.
type AuthToken struct {
	AccessToken string `json:"access_token"`
}

// Device represents a registered WireGuard device on Mullvad.
type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Pubkey      string `json:"pubkey"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
}

// Credentials holds everything needed for Gluetun's .env.
type Credentials struct {
	AccountNumber string
	PrivateKey    string
	PublicKey     string
	IPv4Address   string
	IPv6Address   string
	DeviceID      string
}

// Client is an HTTP client for the Mullvad API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a Mullvad API client with default settings.
func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// NewClientWithBase creates a client with a custom base URL (for testing).
func NewClientWithBase(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// CreateAccount creates a new Mullvad account (no auth required).
func (c *Client) CreateAccount() (*Account, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/accounts/v1/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatus(resp, http.StatusCreated); err != nil {
		return nil, err
	}

	var acct Account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, fmt.Errorf("cannot decode account response: %w", err)
	}
	return &acct, nil
}

// GetToken exchanges an account number for an API access token.
func (c *Client) GetToken(accountNumber string) (*AuthToken, error) {
	body, _ := json.Marshal(map[string]string{"account_number": accountNumber})
	req, err := http.NewRequest("POST", c.baseURL+"/auth/v1/token", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatus(resp, http.StatusOK); err != nil {
		return nil, err
	}

	var token AuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("cannot decode token response: %w", err)
	}
	return &token, nil
}

// RegisterDevice registers a WireGuard public key as a device.
func (c *Client) RegisterDevice(token, pubkey string) (*Device, error) {
	body, _ := json.Marshal(map[string]string{"pubkey": pubkey})
	req, err := http.NewRequest("POST", c.baseURL+"/accounts/v1/devices", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusBadRequest {
		// Mullvad returns 400 when max devices reached
		respBody, _ := io.ReadAll(resp.Body)
		if bytes.Contains(respBody, []byte("MAX_DEVICES_REACHED")) ||
			bytes.Contains(respBody, []byte("too many")) {
			return nil, ErrMaxDevices
		}
		return nil, fmt.Errorf("bad request: %s", string(respBody))
	}

	if err := checkStatus(resp, http.StatusCreated); err != nil {
		return nil, err
	}

	var dev Device
	if err := json.NewDecoder(resp.Body).Decode(&dev); err != nil {
		return nil, fmt.Errorf("cannot decode device response: %w", err)
	}
	return &dev, nil
}

// ListDevices returns all registered devices for the account.
func (c *Client) ListDevices(token string) ([]Device, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/accounts/v1/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatus(resp, http.StatusOK); err != nil {
		return nil, err
	}

	var devices []Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("cannot decode devices response: %w", err)
	}
	return devices, nil
}

// RemoveDevice removes a device by ID.
func (c *Client) RemoveDevice(token, deviceID string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/accounts/v1/devices/"+deviceID, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatus(resp, http.StatusNoContent); err != nil {
		return err
	}
	return nil
}

// GetAccountInfo returns account info for the authenticated user.
func (c *Client) GetAccountInfo(token string) (*Account, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/accounts/v1/accounts/me", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetworkError, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if err := checkStatus(resp, http.StatusOK); err != nil {
		return nil, err
	}

	var acct Account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, fmt.Errorf("cannot decode account response: %w", err)
	}
	return &acct, nil
}

// checkStatus returns a typed error if the HTTP status doesn't match expected.
func checkStatus(resp *http.Response, expected int) error {
	if resp.StatusCode == expected {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return ErrServerError
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// FormatAccountNumber formats a 16-digit account number with spaces (1234 5678 9012 3456).
func FormatAccountNumber(num string) string {
	if len(num) != 16 {
		return num
	}
	return num[0:4] + " " + num[4:8] + " " + num[8:12] + " " + num[12:16]
}
