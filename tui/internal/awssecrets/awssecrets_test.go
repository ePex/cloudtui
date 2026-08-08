package awssecrets

import (
	"context"
	"testing"
)

func TestNewClientRejectsEmptyProfile(t *testing.T) {
	_, err := newClient(context.Background(), "")
	if err == nil {
		t.Fatal("newClient(\"\") error = nil, want an error for a missing profile")
	}
}
