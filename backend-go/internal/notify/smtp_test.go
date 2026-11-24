package notify

import "testing"

func TestEnvelopeSenderUsesBareAddress(t *testing.T) {
	envelope, header, err := envelopeSender("BiletFlow <no-reply@biletflow.local>")
	if err != nil || envelope != "no-reply@biletflow.local" || header != `"BiletFlow" <no-reply@biletflow.local>` {
		t.Fatalf("got %q %q %v", envelope, header, err)
	}
	if envelope, _, _ := envelopeSender("plain@biletflow.local"); envelope != "plain@biletflow.local" {
		t.Fatalf("plain address: %q", envelope)
	}
	if _, _, err := envelopeSender("not an address"); err == nil {
		t.Fatal("invalid sender accepted")
	}
}
