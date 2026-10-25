package devid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// instanceIDAttempts tries to read instance-id, three times over about ten
// seconds (at most 3 × 1.5 s per try plus 2 × 3 s between tries), for the
// short gaps of a host upgrade (renew-when-the-devid-key-is-not-this-tpms D-e).
const instanceIDAttempts = 3

// Seams for the instance-id tests: the pause between tries and the time one
// try, token and instance-id together, may take.
var (
	instanceIDSleep   = func() { time.Sleep(3 * time.Second) }
	instanceIDTimeout = 1500 * time.Millisecond
)

// ReadVMID returns this guest's VM ID as the metadata service at mdsBaseURL
// knows it: /latest/meta-data/instance-id without its "i-" prefix. The service
// requires an IMDSv2 session token for instance-id, so each try asks for one
// with PUT /latest/api/token first. A failed try is repeated; an answer that
// does not start with "i-" is not.
func ReadVMID(mdsBaseURL string) (string, error) {
	var last error
	for attempt := 1; attempt <= instanceIDAttempts; attempt++ {
		if attempt > 1 {
			instanceIDSleep()
		}
		id, err := fetchInstanceID(mdsBaseURL)
		if err != nil {
			last = err
			continue
		}
		vmid, ok := strings.CutPrefix(id, "i-")
		if !ok || vmid == "" {
			return "", fmt.Errorf("instance-id %q does not name a VM (no i- prefix)", id)
		}
		return vmid, nil
	}
	return "", fmt.Errorf("read instance-id after %d tries: %w", instanceIDAttempts, last)
}

func fetchInstanceID(mdsBaseURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), instanceIDTimeout)
	defer cancel()
	client := &http.Client{Transport: directTransport}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, mdsBaseURL+"/latest/api/token", nil)
	if err != nil {
		return "", err
	}
	// The service ignores the TTL today; IMDSv2 clients send it.
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	token, err := doMetadataRequest(client, req)
	if err != nil {
		return "", fmt.Errorf("session token: %w", err)
	}
	if token == "" {
		return "", fmt.Errorf("session token: empty answer")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, mdsBaseURL+"/latest/meta-data/instance-id", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)
	id, err := doMetadataRequest(client, req)
	if err != nil {
		return "", fmt.Errorf("instance-id: %w", err)
	}
	return id, nil
}

func doMetadataRequest(client *http.Client, req *http.Request) (string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return strings.TrimSpace(string(data)), nil
}
