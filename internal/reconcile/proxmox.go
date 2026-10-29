package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ErrDoubtful marks a read the reconciler must not act on: it changes no alias
// and does not count toward a ban.
var ErrDoubtful = errors.New("doubtful read")

// ProxmoxClient reads one cluster's API with an API token.
type ProxmoxClient struct {
	BaseURL string       // e.g. "https://pve.example:8006" (no trailing /api2/json)
	Token   string       // "<user>@<realm>!<tokenid>=<secret>", sent as "Authorization: PVEAPIToken=<Token>"
	HTTP    *http.Client // nil means http.DefaultClient
}

// get fetches path and decodes the "data" member of the answer into out.
// Errors wrap ErrDoubtful and never contain the token.
func (c *ProxmoxClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("%w: building request for %s", ErrDoubtful, path)
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.Token)
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		// The URL error text carries no header, but keep only the cause.
		var msg string
		var ue interface{ Unwrap() error }
		if errors.As(err, &ue) && ue.Unwrap() != nil {
			msg = ue.Unwrap().Error()
		} else {
			msg = err.Error()
		}
		if c.Token != "" {
			msg = strings.ReplaceAll(msg, c.Token, "[token]")
		}
		return fmt.Errorf("%w: GET %s: %s", ErrDoubtful, path, msg)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: GET %s: status %d", ErrDoubtful, path, resp.StatusCode)
	}
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("%w: GET %s: bad JSON", ErrDoubtful, path)
	}
	if err := json.Unmarshal(body.Data, out); err != nil {
		return fmt.Errorf("%w: GET %s: unexpected data", ErrDoubtful, path)
	}
	return nil
}

// CheckPrivileges reads /api2/json/access/permissions and returns nil only when
// the token holds VM.Audit on "/vms" and Pool.Audit on "/pool", each with the
// value 1 (the propagate flag). Otherwise it returns an error wrapping
// ErrDoubtful that names every missing privilege and its path.
func (c *ProxmoxClient) CheckPrivileges(ctx context.Context) error {
	var perms map[string]map[string]float64
	if err := c.get(ctx, "/api2/json/access/permissions", &perms); err != nil {
		return err
	}
	var missing []string
	for _, need := range []struct{ priv, path string }{{"VM.Audit", "/vms"}, {"Pool.Audit", "/pool"}} {
		if perms[need.path][need.priv] != 1 {
			missing = append(missing, "missing "+need.priv+" on "+need.path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: %s", ErrDoubtful, strings.Join(missing, ", "))
	}
	return nil
}

// Guests reads /api2/json/cluster/resources?type=vm and returns the qemu guests
// (entries with "type" == "qemu"; others such as "lxc" are dropped), in the
// order listed.
func (c *ProxmoxClient) Guests(ctx context.Context) ([]Guest, error) {
	var list []struct {
		VMID     json.Number `json:"vmid"`
		Type     string      `json:"type"`
		Pool     string      `json:"pool"`
		Tags     string      `json:"tags"`
		Template float64     `json:"template"`
	}
	if err := c.get(ctx, "/api2/json/cluster/resources?type=vm", &list); err != nil {
		return nil, err
	}
	var guests []Guest
	for _, e := range list {
		if e.Type != "qemu" {
			continue
		}
		id, err := strconv.ParseInt(e.VMID.String(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: GET /api2/json/cluster/resources: bad vmid", ErrDoubtful)
		}
		guests = append(guests, Guest{VMID: strconv.FormatInt(id, 10), Pool: e.Pool, Tags: e.Tags, Template: e.Template != 0})
	}
	return guests, nil
}

// Read checks the privileges and then lists the guests. Any failure, and a
// listing with no qemu guest at all (templates count as guests), is returned
// as an error wrapping ErrDoubtful.
func (c *ProxmoxClient) Read(ctx context.Context) ([]Guest, error) {
	if err := c.CheckPrivileges(ctx); err != nil {
		return nil, err
	}
	guests, err := c.Guests(ctx)
	if err != nil {
		return nil, err
	}
	if len(guests) == 0 {
		return nil, fmt.Errorf("%w: no qemu guest listed", ErrDoubtful)
	}
	return guests, nil
}
