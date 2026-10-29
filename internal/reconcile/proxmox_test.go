package reconcile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const goodPerms = `{"data":{"/":{"VM.Audit":1,"Pool.Audit":1},"/vms":{"VM.Audit":1},"/pool":{"Pool.Audit":1}}}`

// fake serves the two endpoints and records what it saw.
type fake struct {
	perms, res string
	resStatus  int
	auth       string
	resSeen    bool
}

func (f *fake) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api2/json/access/permissions":
			w.Write([]byte(f.perms))
		case "/api2/json/cluster/resources":
			f.resSeen = true
			if f.resStatus != 0 {
				w.WriteHeader(f.resStatus)
				return
			}
			w.Write([]byte(f.res))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestRead(t *testing.T) {
	const secret = "secret"
	tests := []struct {
		name      string
		f         fake
		want      []Guest
		errHas    []string
		wantResrc bool
	}{
		{"a good read", fake{perms: goodPerms, res: `{"data":[
			{"vmid":350,"type":"qemu","pool":"web","tags":"vtpm_mds.io.role.web","template":0,"name":"a","cpu":0.1},
			{"vmid":351,"type":"qemu","template":1},
			{"vmid":100,"type":"lxc","pool":"web"}]}`},
			[]Guest{{VMID: "350", Pool: "web", Tags: "vtpm_mds.io.role.web"}, {VMID: "351", Template: true}}, nil, true},
		{"an empty listing", fake{perms: goodPerms, res: `{"data":[]}`}, nil, []string{"no qemu guest"}, true},
		{"only templates", fake{perms: goodPerms, res: `{"data":[{"vmid":9000,"type":"qemu","template":1}]}`},
			[]Guest{{VMID: "9000", Template: true}}, nil, true},
		{"no rights", fake{perms: `{"data":{}}`, res: `{"data":[]}`}, nil,
			[]string{"VM.Audit on /vms", "Pool.Audit on /pool"}, false},
		{"privilege without propagation", fake{perms: `{"data":{"/vms":{"VM.Audit":1},"/pool":{"Pool.Audit":0}}}`, res: `{"data":[]}`},
			nil, []string{"Pool.Audit on /pool"}, false},
		{"server error", fake{perms: goodPerms, resStatus: 500}, nil, []string{"500"}, true},
		{"vmid as number", fake{perms: goodPerms, res: `{"data":[{"vmid":1234,"type":"qemu"}]}`},
			[]Guest{{VMID: "1234"}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.f.server()
			defer srv.Close()
			c := &ProxmoxClient{BaseURL: srv.URL, Token: "user@pve!probe=" + secret}
			got, err := c.Read(context.Background())
			if tt.errHas != nil {
				if !errors.Is(err, ErrDoubtful) {
					t.Fatalf("err = %v, want ErrDoubtful", err)
				}
				for _, s := range tt.errHas {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("err %q lacks %q", err, s)
					}
				}
				if strings.Contains(err.Error(), secret) {
					t.Errorf("err %q contains the token secret", err)
				}
			} else {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("guests = %+v, want %+v", got, tt.want)
				}
			}
			if tt.f.resSeen != tt.wantResrc {
				t.Errorf("resources requested = %v, want %v", tt.f.resSeen, tt.wantResrc)
			}
			if want := "PVEAPIToken=user@pve!probe=" + secret; tt.f.auth != want {
				t.Errorf("Authorization = %q, want %q", tt.f.auth, want)
			}
		})
	}
}

func TestReadUnreachable(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		c := &ProxmoxClient{BaseURL: url, Token: "user@pve!probe=secret"}
		_, err := c.Read(context.Background())
		if !errors.Is(err, ErrDoubtful) {
			t.Fatalf("err = %v, want ErrDoubtful", err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("err %q contains the token secret", err)
		}
	})
}
