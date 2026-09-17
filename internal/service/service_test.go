package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"n8n-cli/internal/client"
)

// nodeSettings are the per-node settings a user can set in the n8n editor that
// the parser does not model as typed fields. A save must not drop them.
var nodeSettings = map[string]interface{}{
	"retryOnFail":      true,
	"maxTries":         float64(4),
	"waitBetweenTries": float64(2500),
	"continueOnFail":   true,
	"onError":          "continueErrorOutput",
	"alwaysOutputData": true,
	"executeOnce":      true,
	"notesInFlow":      true,
	"notes":            "keep me",
	"disabled":         true,
}

func workflowFixture() map[string]interface{} {
	nodes := []interface{}{}
	for _, name := range []string{"Webhook", "HTTP Request"} {
		node := map[string]interface{}{
			"id":          "id-" + name,
			"name":        name,
			"type":        "n8n-nodes-base.noOp",
			"typeVersion": float64(1),
			"position":    []interface{}{float64(0), float64(0)},
			"parameters":  map[string]interface{}{},
			"webhookId":   "webhook-" + name,
		}
		for k, v := range nodeSettings {
			node[k] = v
		}
		nodes = append(nodes, node)
	}
	return map[string]interface{}{
		"id":          "wf-1",
		"name":        "Round Trip",
		"active":      false,
		"nodes":       nodes,
		"connections": map[string]interface{}{"Webhook": map[string]interface{}{"main": []interface{}{[]interface{}{map[string]interface{}{"node": "HTTP Request", "type": "main", "index": float64(0)}}}}},
		"settings":    map[string]interface{}{"executionOrder": "v1"},
		"createdAt":   "2026-01-01T00:00:00Z",
		"updatedAt":   "2026-01-02T00:00:00Z",
	}
}

func TestRenameNodePreservesNodeSettings(t *testing.T) {
	var putBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read PUT body: %v", err)
			}
			if err := json.Unmarshal(data, &putBody); err != nil {
				t.Errorf("parse PUT body: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(workflowFixture())
	}))
	defer srv.Close()

	svc := New(client.New(srv.URL, "test-key", 5*time.Second))
	node, err := svc.RenameNode("wf-1", "Webhook", "Entry Point", false)
	if err != nil {
		t.Fatalf("rename node: %v", err)
	}
	if node.Name != "Entry Point" {
		t.Fatalf("expected renamed node, got %q", node.Name)
	}
	if putBody == nil {
		t.Fatal("expected a PUT request to the n8n API")
	}

	nodes, ok := putBody["nodes"].([]interface{})
	if !ok || len(nodes) != 2 {
		t.Fatalf("expected 2 nodes in PUT body, got %v", putBody["nodes"])
	}
	names := map[string]bool{}
	for i, rn := range nodes {
		n, ok := rn.(map[string]interface{})
		if !ok {
			t.Fatalf("node %d is not an object: %v", i, rn)
		}
		names[n["name"].(string)] = true
		for k, want := range nodeSettings {
			got, present := n[k]
			if !present {
				t.Errorf("node %v: setting %q was dropped from the PUT body", n["name"], k)
				continue
			}
			if got != want {
				t.Errorf("node %v: setting %q = %v, want %v", n["name"], k, got, want)
			}
		}
		if n["webhookId"] != "webhook-"+i2name(i) {
			t.Errorf("node %v: webhookId = %v", n["name"], n["webhookId"])
		}
	}
	if !names["Entry Point"] || !names["HTTP Request"] {
		t.Errorf("expected renamed and untouched node in PUT body, got %v", names)
	}

	for _, k := range []string{"id", "createdAt", "updatedAt"} {
		if _, ok := putBody[k]; ok {
			t.Errorf("read-only workflow field %q must not be sent on PUT", k)
		}
	}
}

func i2name(i int) string {
	if i == 0 {
		return "Webhook"
	}
	return "HTTP Request"
}
