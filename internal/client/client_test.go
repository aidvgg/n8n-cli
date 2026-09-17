package client

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

func TestSanitizeWorkflowBody(t *testing.T) {
	body := map[string]interface{}{
		"name": "Test Workflow",
		"nodes": []interface{}{
			map[string]interface{}{
				"id":          "node-1",
				"name":        "Webhook",
				"type":        "n8n-nodes-base.webhook",
				"typeVersion": 2,
				"position":    []interface{}{100, 200},
				"parameters":  map[string]interface{}{"path": "test"},
				"webhookId":   "stable-webhook-id",
			},
		},
		"connections": map[string]interface{}{},
		"settings": map[string]interface{}{
			"executionOrder": "v1",
			"availableInMCP": false,
			"callerPolicy":   "workflowsFromSameOwner",
		},
		"staticData":   "{}",
		"pinData":      map[string]interface{}{},
		"versionId":    "abc-123",
		"id":           "wf-123",
		"createdAt":    "2026-01-01T00:00:00Z",
		"updatedAt":    "2026-03-13T00:00:00Z",
		"triggerCount": float64(5),
		"meta":         map[string]interface{}{},
		"active":       false,
		"tags":         []interface{}{},
		"description":  "desc",
	}

	clean := sanitizeWorkflowBody(body)

	expected := []string{"connections", "name", "nodes", "settings"}
	sort.Strings(expected)
	var got []string
	for k := range clean {
		got = append(got, k)
	}
	sort.Strings(got)
	if len(got) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(got), got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("key %d: expected %q, got %q", i, expected[i], got[i])
		}
	}

	nodes := clean["nodes"].([]interface{})
	node := nodes[0].(map[string]interface{})
	if got := node["webhookId"]; got != "stable-webhook-id" {
		t.Errorf("expected webhookId to be preserved, got %v", got)
	}
	for _, k := range []string{"id", "name", "type", "typeVersion", "position", "parameters"} {
		if _, ok := node[k]; !ok {
			t.Errorf("expected node field %q to be preserved", k)
		}
	}

	settings := clean["settings"].(map[string]interface{})
	if _, ok := settings["availableInMCP"]; ok {
		t.Errorf("availableInMCP should be stripped from settings")
	}
	if _, ok := settings["callerPolicy"]; ok {
		t.Errorf("callerPolicy should be stripped from settings")
	}
	if v, ok := settings["executionOrder"]; !ok || v != "v1" {
		t.Errorf("expected settings.executionOrder to be preserved, got %v", settings["executionOrder"])
	}
}

func TestSanitizeWorkflowBodyMinimal(t *testing.T) {
	body := map[string]interface{}{
		"name":        "Minimal",
		"nodes":       []interface{}{},
		"connections": map[string]interface{}{},
	}
	clean := sanitizeWorkflowBody(body)
	if len(clean) != 3 {
		t.Errorf("expected 3 keys, got %d: %v", len(clean), clean)
	}
}

func TestRequestTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	c := New(srv.URL, "test-key", 50*time.Millisecond)
	done := make(chan error, 1)
	go func() {
		_, err := c.GetWorkflow("wf-1")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not time out")
	}
}
