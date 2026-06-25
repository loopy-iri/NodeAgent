package shared

import (
	"encoding/json"
	"testing"
)

func TestSharableInbounds(t *testing.T) {
	cfg := `{
		"inbounds": [
			{"tag":"vless-in","port":443,"protocol":"vless"},
			{"tag":"admin-in","port":10085,"protocol":"dokodemo-door"}
		],
		"outbounds": [{"tag":"direct","protocol":"freedom"}]
	}`

	// No force-inbounds: return all inbounds, drop outbounds.
	m := &Manager{config: cfg}
	out, err := m.SharableInbounds()
	if err != nil {
		t.Fatalf("SharableInbounds: %v", err)
	}
	var doc struct {
		Inbounds  []map[string]any `json:"inbounds"`
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Inbounds) != 2 {
		t.Fatalf("want 2 inbounds, got %d", len(doc.Inbounds))
	}
	if doc.Outbounds != nil {
		t.Fatalf("outbounds must not be shared")
	}

	// Force-inbounds narrows to the matching tag only.
	m2 := &Manager{config: cfg, forceInbounds: []string{"vless-in"}}
	out2, err := m2.SharableInbounds()
	if err != nil {
		t.Fatalf("SharableInbounds(forced): %v", err)
	}
	_ = json.Unmarshal([]byte(out2), &doc)
	if len(doc.Inbounds) != 1 || doc.Inbounds[0]["tag"] != "vless-in" {
		t.Fatalf("force-inbounds did not narrow correctly: %s", out2)
	}

	// Empty config yields an empty inbounds doc, not an error.
	m3 := &Manager{}
	if out3, err := m3.SharableInbounds(); err != nil || out3 != `{"inbounds":[]}` {
		t.Fatalf("empty config: got %q err %v", out3, err)
	}
}
