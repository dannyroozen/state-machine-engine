package domain

import (
	"encoding/json"
	"testing"
)

func TestResponseEnvelope_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	resp := ResponseEnvelope{
		SessionID: "s1",
		State:     "start",
		Output:    json.RawMessage(`{"ok":true}`),
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ResponseEnvelope
	if err = json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.SessionID != "s1" || decoded.State != "start" || string(decoded.Output) != `{"ok":true}` {
		t.Fatalf("unexpected decoded response: %+v", decoded)
	}
}
