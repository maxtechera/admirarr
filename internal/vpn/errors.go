package vpn

import "errors"

// Sentinel errors for Mullvad API operations.
var (
	ErrUnauthorized = errors.New("unauthorized: invalid account number or expired token")
	ErrRateLimited  = errors.New("rate limited: too many requests to Mullvad API")
	ErrServerError  = errors.New("mullvad API server error")
	ErrMaxDevices   = errors.New("maximum number of devices reached (5)")
	ErrNetworkError = errors.New("network error: cannot reach Mullvad API")
)
