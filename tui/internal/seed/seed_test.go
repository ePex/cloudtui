package seed

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeSender struct {
	bodies  []string
	failAt  int // 1-indexed message number to fail on; 0 = never fail
	sendErr error
}

func (f *fakeSender) SendMessage(_ context.Context, _, body string) error {
	n := len(f.bodies) + 1
	if f.failAt != 0 && n == f.failAt {
		return f.sendErr
	}
	f.bodies = append(f.bodies, body)
	return nil
}

func TestRunSendsCountMessages(t *testing.T) {
	sender := &fakeSender{}
	if err := Run(context.Background(), sender, "orders", 5, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := len(sender.bodies), 5; got != want {
		t.Fatalf("sent %d messages, want %d", got, want)
	}
}

func TestRunMessagesAreValidJSONWithSequentialIDs(t *testing.T) {
	sender := &fakeSender{}
	if err := Run(context.Background(), sender, "orders", 3, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for i, body := range sender.bodies {
		var decoded struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatalf("message %d: not valid JSON: %v (body = %q)", i, err, body)
		}
		if want := i + 1; decoded.ID != want {
			t.Errorf("message %d: id = %d, want %d", i, decoded.ID, want)
		}
	}
}

func TestRunReportsProgress(t *testing.T) {
	sender := &fakeSender{}
	var gotSent, gotTotal []int
	err := Run(context.Background(), sender, "orders", 3, func(sent, total int) {
		gotSent = append(gotSent, sent)
		gotTotal = append(gotTotal, total)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	wantSent := []int{1, 2, 3}
	if len(gotSent) != len(wantSent) {
		t.Fatalf("progress called %d times, want %d", len(gotSent), len(wantSent))
	}
	for i, want := range wantSent {
		if gotSent[i] != want || gotTotal[i] != 3 {
			t.Errorf("progress call %d = (%d, %d), want (%d, 3)", i, gotSent[i], gotTotal[i], want)
		}
	}
}

func TestRunStopsAndReturnsErrorOnSendFailure(t *testing.T) {
	wantErr := errors.New("broker unreachable")
	sender := &fakeSender{failAt: 2, sendErr: wantErr}

	err := Run(context.Background(), sender, "orders", 5, nil)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want wrapping %v", err, wantErr)
	}
	if got, want := len(sender.bodies), 1; got != want {
		t.Errorf("sent %d messages before stopping, want %d", got, want)
	}
}
