package api

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowsDirectoryCompletionIgnoresCase(t *testing.T) {
	root := t.TempDir()
	want := []string{filepath.Join(root, "PROJECT alpha"), filepath.Join(root, "Project beta")}
	for _, dir := range want {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{}
	r := httptest.NewRecorder()
	s.dirs(r, httptest.NewRequest("GET", "/api/fs/dirs?prefix="+url.QueryEscape(filepath.Join(root, "project")), nil))
	var got []string
	if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil || r.Code != 200 || !reflect.DeepEqual(got, want) {
		t.Fatalf("completion: status=%d, paths=%q, error=%v", r.Code, got, err)
	}
}
