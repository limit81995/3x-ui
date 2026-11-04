package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tclimit"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// This exercises real HTTP binding, service persistence and response encoding.
// Run the compiled test in a disposable namespace with NET_ADMIN to enable it.
func TestInboundSpeedLimitAPI(t *testing.T) {
	if err := tclimit.Available(); err != nil {
		t.Skip(err)
	}
	gin.SetMode(gin.TestMode)
	folder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", folder)
	dbtest.InitDB(t, filepath.Join(folder, "x-ui.db"))
	previous := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(previous) })
	if err := database.GetDB().Create(&model.ApiToken{Name: "tc-test", Token: crypto.HashTokenSHA256("tc-test"), Enabled: true, Scope: model.ApiScopeAdmin}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := &APIController{}
	group := router.Group("/panel/api")
	group.Use(auth.checkAPIAuth, auth.enforceTokenScope)
	NewInboundController(group.Group("/inbounds"))
	for i, contentType := range []string{"application/json", "application/x-www-form-urlencoded"} {
		t.Run(contentType, func(t *testing.T) {
			call := func(path string, body map[string]any, success bool) map[string]any {
				t.Helper()
				var raw []byte
				if contentType == "application/json" {
					raw, _ = json.Marshal(body)
				} else {
					form := url.Values{}
					for k, v := range body {
						form.Set(k, fmt.Sprint(v))
					}
					raw = []byte(form.Encode())
				}
				method := http.MethodPost
				if body == nil {
					method = http.MethodGet
				}
				req := httptest.NewRequest(method, "/panel/api/inbounds/"+path, bytes.NewReader(raw))
				req.Header.Set("Content-Type", contentType)
				req.Header.Set("Authorization", "Bearer tc-test")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				var envelope struct {
					Success bool
					Obj     map[string]any
					Msg     string
				}
				if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err, response.Body.String())
				}
				if envelope.Success != success {
					t.Fatalf("%s: %s", path, response.Body)
				}
				return envelope.Obj
			}
			body := map[string]any{"protocol": "vless", "port": 20100 + i, "enable": false, "speedLimit": 100, "settings": `{"clients":[],"decryption":"none"}`, "streamSettings": `{"network":"tcp","security":"none"}`}
			created := call("add", body, true)
			id := int(created["id"].(float64))
			if created["speedLimit"] != float64(100) {
				t.Fatal(created)
			}
			for _, rate := range []int{50, 0} {
				body["speedLimit"] = rate
				updated := call(fmt.Sprintf("update/%d", id), body, true)
				if updated["speedLimit"] != float64(rate) {
					t.Fatal(updated)
				}
				got := call(fmt.Sprintf("get/%d", id), nil, true)
				if got["speedLimit"] != float64(rate) {
					t.Fatal(got)
				}
			}
			body["speedLimit"] = -1
			call(fmt.Sprintf("update/%d", id), body, false)
			got := call(fmt.Sprintf("get/%d", id), nil, true)
			if got["speedLimit"] != float64(0) {
				t.Fatal("rejected update changed the stored cap")
			}
		})
	}
}
