package api

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/session"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProtocolAndSecurity(t *testing.T) {
	c, err := config.Load(t.TempDir()+"/absent", false)
	if err != nil {
		t.Fatal(err)
	}
	c.Token = "secret"
	c.DefaultCwd = t.TempDir()
	if err = c.Validate(); err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(c.SessionOptions())
	server := httptest.NewServer(New(c, m))
	defer server.Close()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	}()
	request := func(method, path, body, token, origin string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for _, row := range []struct {
		token, origin string
		want          int
	}{{"", "", 401}, {"wrong", "", 401}, {"secret", "http://evil.example", 403}, {"secret", "", 200}} {
		res := request("GET", "/api/health", "", row.token, row.origin)
		_ = res.Body.Close()
		if res.StatusCode != row.want {
			t.Fatalf("status %d want %d", res.StatusCode, row.want)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dial := func(path string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+path+"?token=secret", nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.CloseNow() })
		return conn
	}
	list := dial("/ws/sessions")
	_, raw, err := list.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Type     string
		Sessions []session.Info
	}
	if err = json.Unmarshal(raw, &snapshot); err != nil || snapshot.Type != "snapshot" || snapshot.Sessions == nil {
		t.Fatalf("empty snapshot: %s", raw)
	}
	res := request("POST", "/api/sessions", `{"preset":"shell"}`, "secret", "")
	if res.StatusCode != 201 {
		t.Fatal(res.Status)
	}
	var info session.Info
	_ = json.NewDecoder(res.Body).Decode(&info)
	_ = res.Body.Close()
	res = request("DELETE", "/api/sessions/"+info.ID+"?remove=1", "", "secret", "")
	_ = res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatal("running removal must conflict")
	}
	conn := dial("/ws/s/" + info.ID)
	kind, p, err := conn.Read(ctx)
	if err != nil || kind != websocket.MessageBinary || p[0] != 1 {
		t.Fatalf("initial frame %v %v %v", kind, p, err)
	}
	if err = conn.Write(ctx, websocket.MessageBinary, session.Frame(0, []byte("printf 'FINAL_OUTPUT_MARKER\\n'; exit 6\n"))); err != nil {
		t.Fatal(err)
	}
	for {
		_, p, err = conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if p[0] == 3 {
			break
		}
	}
	again := dial("/ws/s/" + info.ID)
	_, p, err = again.Read(ctx)
	if err != nil || p[0] != 1 || !strings.Contains(string(p[1:]), "FINAL_OUTPUT_MARKER") {
		t.Fatalf("final replay %q %v", p, err)
	}
	for {
		_, p, err = again.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if p[0] == 3 {
			if string(p[1:]) != `{"exitCode":6}` {
				t.Fatal(string(p))
			}
			break
		}
	}
	_, _, err = again.Read(ctx)
	if websocket.CloseStatus(err) != 4410 {
		t.Fatalf("exit close: %v", err)
	}
	res = request("GET", "/api/sessions/"+info.ID+"/buffer", "", "secret", "")
	buffer, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if !strings.Contains(string(buffer), "FINAL_OUTPUT_MARKER") {
		t.Fatal("buffer lost output")
	}
	res = request("DELETE", "/api/sessions/"+info.ID+"?remove=1", "", "secret", "")
	_ = res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatal(res.Status)
	}
}
