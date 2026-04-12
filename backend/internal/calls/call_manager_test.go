package calls

import "testing"

func TestDeclineCallInvokesDeclinedCallbackWithType(t *testing.T) {
	cm := NewCallManager(nil, 0)

	callID := cm.CreateCallSession("FORCED_ENTRY", "")

	var gotID string
	var gotType string
	cm.SetOnDeclinedCall(func(id string, callType string) {
		gotID = id
		gotType = callType
	})

	if ok := cm.DeclineCall(callID); !ok {
		t.Fatal("expected decline call to succeed")
	}

	if gotID != callID {
		t.Fatalf("expected declined callback callID %q, got %q", callID, gotID)
	}
	if gotType != "FORCED_ENTRY" {
		t.Fatalf("expected declined callback call type %q, got %q", "FORCED_ENTRY", gotType)
	}
}
