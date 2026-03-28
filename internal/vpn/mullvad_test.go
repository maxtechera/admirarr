package vpn

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	srv := httptest.NewServer(handler)
	client := NewClientWithBase(srv.URL)
	return srv, client
}

func TestCreateAccount(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/accounts/v1/accounts" {
			http.Error(w, "not found", 404)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Account{
			Number:    "1234567890123456",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})
	})
	defer srv.Close()

	acct, err := client.CreateAccount()
	if err != nil {
		t.Fatalf("CreateAccount() error: %v", err)
	}
	if acct.Number != "1234567890123456" {
		t.Errorf("account number = %q, want %q", acct.Number, "1234567890123456")
	}
}

func TestGetToken(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/auth/v1/token" {
			http.Error(w, "not found", 404)
			return
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["account_number"] != "1234567890123456" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(AuthToken{AccessToken: "test-token-123"})
	})
	defer srv.Close()

	token, err := client.GetToken("1234567890123456")
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token.AccessToken != "test-token-123" {
		t.Errorf("token = %q, want %q", token.AccessToken, "test-token-123")
	}

	// Invalid account should return unauthorized
	_, err = client.GetToken("0000000000000000")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRegisterDevice(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/accounts/v1/devices" {
			http.Error(w, "not found", 404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Device{
			ID:          "dev-123",
			Name:        "admirarr",
			Pubkey:      "testpubkey==",
			IPv4Address: "10.64.0.1/32",
			IPv6Address: "fc00::1/128",
		})
	})
	defer srv.Close()

	dev, err := client.RegisterDevice("test-token", "testpubkey==")
	if err != nil {
		t.Fatalf("RegisterDevice() error: %v", err)
	}
	if dev.IPv4Address != "10.64.0.1/32" {
		t.Errorf("IPv4 = %q, want %q", dev.IPv4Address, "10.64.0.1/32")
	}
}

func TestMaxDevicesError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"MAX_DEVICES_REACHED"}`))
	})
	defer srv.Close()

	_, err := client.RegisterDevice("token", "pubkey")
	if !errors.Is(err, ErrMaxDevices) {
		t.Errorf("expected ErrMaxDevices, got: %v", err)
	}
}

func TestListDevices(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/accounts/v1/devices" {
			http.Error(w, "not found", 404)
			return
		}
		_ = json.NewEncoder(w).Encode([]Device{
			{ID: "1", Name: "dev1", Pubkey: "key1"},
			{ID: "2", Name: "dev2", Pubkey: "key2"},
		})
	})
	defer srv.Close()

	devices, err := client.ListDevices("token")
	if err != nil {
		t.Fatalf("ListDevices() error: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("got %d devices, want 2", len(devices))
	}
}

func TestRemoveDevice(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/accounts/v1/devices/dev-123" {
			http.Error(w, "not found", 404)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	err := client.RemoveDevice("token", "dev-123")
	if err != nil {
		t.Fatalf("RemoveDevice() error: %v", err)
	}
}

func TestFormatAccountNumber(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"1234567890123456", "1234 5678 9012 3456"},
		{"short", "short"},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatAccountNumber(tt.input)
		if got != tt.want {
			t.Errorf("FormatAccountNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRateLimited(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()

	_, err := client.CreateAccount()
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
}
