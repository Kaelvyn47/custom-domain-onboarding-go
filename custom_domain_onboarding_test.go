package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnboardUsesZoneIDAndRegistersWebhook(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/dns/domain/add":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"zone_id": "zone-7"}})
		case "/dns/record/upsert":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["zone_id"] != "zone-7" {
				t.Errorf("record used %v", body["zone_id"])
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{}})
		case "/account/webhooks/register":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]string{"id": "wh-2"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, Key: "test", HTTP: srv.Client()}
	got, err := Onboard(context.Background(), c, "shop.example", "https://shop.example/hooks", "token")
	if err != nil {
		t.Fatal(err)
	}
	if got.ZoneID != "zone-7" || got.WebhookID != "wh-2" {
		t.Fatalf("result %+v", got)
	}
	if len(paths) != 3 {
		t.Fatalf("paths %v", paths)
	}
}
