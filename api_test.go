package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildServicesResponse(t *testing.T) {
	resp := buildServicesResponse("mirrors.0e7.cn")
	if resp.RequestHost != "mirrors.0e7.cn" {
		t.Fatalf("unexpected request host: %s", resp.RequestHost)
	}
	if resp.Sources == nil || resp.Proxy == nil || resp.HostAliases == nil {
		t.Fatal("response slices should be initialized")
	}
}

func TestHandleAPIServices(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	req.Host = "mirrors.0e7.cn"
	recorder := httptest.NewRecorder()
	handleAPI(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload apiServicesResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.RequestHost != "mirrors.0e7.cn" {
		t.Fatalf("expected requestHost mirrors.0e7.cn, got %s", payload.RequestHost)
	}
	if payload.Sources == nil || payload.Proxy == nil || payload.HostAliases == nil {
		t.Fatal("expected initialized slices")
	}
}
