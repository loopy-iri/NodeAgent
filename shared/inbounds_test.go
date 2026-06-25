package shared

import (
	"encoding/json"
	"strings"
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

func TestSharableInboundsRedactsRealityPrivateKey(t *testing.T) {
	// A valid X25519 private key (base64 raw-url) and its expected public key.
	priv := "WMHfHKQjPbTNbeKr0PmCvFjyhZjUbqI0c5q3oRZ7Ykg"
	wantPub, ok := realityPublicKey(priv)
	if !ok {
		t.Fatal("could not derive public key from test private key")
	}

	cfg := `{"inbounds":[{"tag":"vless-in","port":443,"protocol":"vless",
		"streamSettings":{"network":"tcp","security":"reality",
		"realitySettings":{"serverNames":["www.microsoft.com"],"privateKey":"` + priv + `","shortIds":["ab"]}}}]}`

	m := &Manager{config: cfg}
	out, err := m.SharableInbounds()
	if err != nil {
		t.Fatalf("SharableInbounds: %v", err)
	}
	if strings.Contains(out, priv) {
		t.Fatalf("private key leaked in shared inbounds: %s", out)
	}
	if !strings.Contains(out, wantPub) {
		t.Fatalf("expected derived public key %q in output: %s", wantPub, out)
	}
}
