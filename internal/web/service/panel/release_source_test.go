package panel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestPanelReleaseSourceDownloads(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	dbtest.InitDB(t, config.GetDBPath())
	for _, tc := range []struct{ name, repo, ref, wantRepo, wantRef string }{
		{"default fork", "", "", "limit81995/3x-ui", "main"},
		{"explicit source", "example/panel", "release/test", "example/panel", "release/test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XUI_RELEASE_REPO", tc.repo)
			t.Setenv("XUI_RELEASE_REF", tc.ref)
			var mu sync.Mutex
			var requests []string
			missing := false
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, r.Host+r.URL.Path)
				if missing {
					http.NotFound(w, r)
					return
				}
				switch r.Host + r.URL.Path {
				case "api.github.com/repos/" + tc.wantRepo + "/releases/latest":
					fmt.Fprint(w, `{"tag_name":"v9.8.7"}`)
				case "api.github.com/repos/" + tc.wantRepo + "/releases/tags/dev-latest":
					fmt.Fprint(w, `{"tag_name":"dev-latest","body":"commit=12345678abcdef00"}`)
				case "raw.githubusercontent.com/" + tc.wantRepo + "/" + tc.wantRef + "/update.sh":
					fmt.Fprint(w, "#!/bin/bash\nexit 0\n")
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			transport := srv.Client().Transport.(*http.Transport).Clone()
			transport.TLSClientConfig.ServerName = "example.com"
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			}
			previous := http.DefaultTransport
			http.DefaultTransport = transport
			defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()

			version, err := fetchLatestPanelVersion()
			if err != nil || version != "v9.8.7" {
				t.Fatalf("stable release = %q, %v; requests: %v", version, err, requests)
			}
			release, err := fetchPanelRelease("dev-latest")
			if err != nil || extractReleaseCommit(release) != "12345678abcdef00" {
				t.Fatalf("dev release = %+v, %v", release, err)
			}
			file, err := downloadPanelUpdater()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file)
			body, err := os.ReadFile(file)
			if err != nil || string(body) != "#!/bin/bash\nexit 0\n" {
				t.Fatalf("updater contents = %q, %v", body, err)
			}

			mu.Lock()
			missing = true
			requests = nil
			mu.Unlock()
			if _, err := fetchLatestPanelVersion(); err == nil {
				t.Fatal("missing release must fail")
			}
			if _, err := fetchPanelRelease("dev-latest"); err == nil {
				t.Fatal("missing dev release must fail")
			}
			if _, err := downloadPanelUpdater(); err == nil {
				t.Fatal("missing updater must fail")
			}
			mu.Lock()
			defer mu.Unlock()
			if len(requests) != 3 {
				t.Fatalf("unexpected fallback requests: %v", requests)
			}
			for _, request := range requests {
				if !strings.Contains(request, tc.wantRepo+"/") {
					t.Fatalf("request escaped the selected repository: %s", request)
				}
			}
		})
	}
}

func TestPanelReleaseSourceRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"XUI_RELEASE_REPO", "owner/repo?redirect=upstream"},
		{"XUI_RELEASE_REPO", "https://github.com/owner/repo"},
		{"XUI_RELEASE_REF", "../main"},
		{"XUI_RELEASE_REF", "dev?raw=true"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("XUI_RELEASE_REPO", "")
			t.Setenv("XUI_RELEASE_REF", "")
			t.Setenv(tc.key, tc.value)
			if _, err := fetchPanelRelease(""); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("release lookup accepted invalid configuration: %v", err)
			}
			if _, err := downloadPanelUpdater(); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("updater download accepted invalid configuration: %v", err)
			}
		})
	}
}
