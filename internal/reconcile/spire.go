package reconcile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

// pageSize is how many entries or agents one list call asks for.
const pageSize = 500

// SPIRE talks to a SPIRE server through its local admin socket. The socket
// itself is the credential: whoever can open it is an admin.
type SPIRE struct {
	conn    *grpc.ClientConn
	entries entryv1.EntryClient
	agents  agentv1.AgentClient
	td      string
}

// DialSPIRE connects to the admin socket of the server of trust domain td.
func DialSPIRE(socket, td string) (*SPIRE, error) {
	conn, err := grpc.NewClient("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("spire socket %s: %w", socket, err)
	}
	return &SPIRE{conn: conn, entries: entryv1.NewEntryClient(conn), agents: agentv1.NewAgentClient(conn), td: td}, nil
}

// Close closes the connection.
func (s *SPIRE) Close() error { return s.conn.Close() }

func (s *SPIRE) id(path string) *types.SPIFFEID {
	return &types.SPIFFEID{TrustDomain: s.td, Path: path}
}

func idString(id *types.SPIFFEID) string {
	if id == nil {
		return ""
	}
	return "spiffe://" + id.TrustDomain + id.Path
}

func selectorStrings(sel []*types.Selector) []string {
	out := make([]string, 0, len(sel))
	for _, x := range sel {
		out = append(out, x.Type+":"+x.Value)
	}
	return out
}

// Entries lists every entry whose parent is the server, that is every node
// alias, page by page. The entry API has no prefix filter; Diff filters.
func (s *SPIRE) Entries(ctx context.Context) ([]Entry, error) {
	var out []Entry
	token := ""
	for {
		resp, err := s.entries.ListEntries(ctx, &entryv1.ListEntriesRequest{
			Filter:    &entryv1.ListEntriesRequest_Filter{ByParentId: s.id("/spire/server")},
			PageSize:  pageSize,
			PageToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list entries: %w", err)
		}
		for _, e := range resp.Entries {
			out = append(out, Entry{
				ID:        e.Id,
				ParentID:  idString(e.ParentId),
				SPIFFEID:  idString(e.SpiffeId),
				Selectors: selectorStrings(e.Selectors),
				Default: !e.Admin && !e.Downstream && len(e.DnsNames) == 0 &&
					len(e.FederatesWith) == 0 && e.X509SvidTtl == 0 && e.JwtSvidTtl == 0 &&
					e.Hint == "" && e.ExpiresAt == 0 && !e.StoreSvid,
			})
		}
		if resp.NextPageToken == "" {
			return out, nil
		}
		token = resp.NextPageToken
	}
}

// Create creates the aliases as node aliases parented on the server, with
// every other field at its default.
func (s *SPIRE) Create(ctx context.Context, aliases []Alias) error {
	if len(aliases) == 0 {
		return nil
	}
	req := &entryv1.BatchCreateEntryRequest{}
	for _, a := range aliases {
		path, ok := strings.CutPrefix(a.SPIFFEID, "spiffe://"+s.td)
		if !ok {
			return fmt.Errorf("alias %s is outside trust domain %s", a.SPIFFEID, s.td)
		}
		e := &types.Entry{ParentId: s.id("/spire/server"), SpiffeId: s.id(path)}
		for _, sel := range a.Selectors {
			typ, val, _ := strings.Cut(sel, ":")
			e.Selectors = append(e.Selectors, &types.Selector{Type: typ, Value: val})
		}
		req.Entries = append(req.Entries, e)
	}
	resp, err := s.entries.BatchCreateEntry(ctx, req)
	if err != nil {
		return fmt.Errorf("create entries: %w", err)
	}
	var errs []error
	for i, r := range resp.Results {
		if c := codes.Code(r.Status.GetCode()); c != codes.OK {
			errs = append(errs, fmt.Errorf("create %s: %s: %s", aliases[i].SPIFFEID, c, r.Status.GetMessage()))
		}
	}
	return errors.Join(errs...)
}

// Delete deletes the entries with the given IDs.
func (s *SPIRE) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	resp, err := s.entries.BatchDeleteEntry(ctx, &entryv1.BatchDeleteEntryRequest{Ids: ids})
	if err != nil {
		return fmt.Errorf("delete entries: %w", err)
	}
	var errs []error
	for _, r := range resp.Results {
		if c := codes.Code(r.Status.GetCode()); c != codes.OK && c != codes.NotFound {
			errs = append(errs, fmt.Errorf("delete %s: %s: %s", r.Id, c, r.Status.GetMessage()))
		}
	}
	return errors.Join(errs...)
}

// Agents lists every agent attested with tpm_devid, with its selectors.
func (s *SPIRE) Agents(ctx context.Context) ([]Agent, error) {
	var out []Agent
	token := ""
	for {
		resp, err := s.agents.ListAgents(ctx, &agentv1.ListAgentsRequest{
			Filter:    &agentv1.ListAgentsRequest_Filter{ByAttestationType: "tpm_devid"},
			PageSize:  pageSize,
			PageToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list agents: %w", err)
		}
		for _, a := range resp.Agents {
			out = append(out, Agent{ID: idString(a.Id), Selectors: selectorStrings(a.Selectors), Banned: a.Banned})
		}
		if resp.NextPageToken == "" {
			return out, nil
		}
		token = resp.NextPageToken
	}
}

// Ban bans the agent with the given SPIFFE ID.
func (s *SPIRE) Ban(ctx context.Context, agentID string) error {
	path, ok := strings.CutPrefix(agentID, "spiffe://"+s.td)
	if !ok {
		return fmt.Errorf("agent %s is outside trust domain %s", agentID, s.td)
	}
	if _, err := s.agents.BanAgent(ctx, &agentv1.BanAgentRequest{Id: s.id(path)}); err != nil {
		return fmt.Errorf("ban %s: %w", agentID, err)
	}
	return nil
}
