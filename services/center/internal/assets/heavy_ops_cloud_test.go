package assets

import "testing"

func TestShouldResumeCD2Upload(t *testing.T) {
	paused := "Pause"
	if !shouldResumeCD2Upload("QUEUED", &paused) {
		t.Fatalf("expected queued paused upload to resume")
	}

	active := "Transfer"
	if shouldResumeCD2Upload("QUEUED", &active) {
		t.Fatalf("did not expect active upload to resume")
	}

	if shouldResumeCD2Upload("RUNNING", &paused) {
		t.Fatalf("did not expect non-queued item to resume")
	}
}

func TestShouldResumeAria2Download(t *testing.T) {
	paused := "paused"
	if !shouldResumeAria2Download("QUEUED", &paused) {
		t.Fatalf("expected queued paused download to resume")
	}

	active := "active"
	if shouldResumeAria2Download("QUEUED", &active) {
		t.Fatalf("did not expect active download to resume")
	}

	if shouldResumeAria2Download("RUNNING", &paused) {
		t.Fatalf("did not expect non-queued item to resume")
	}
}
