package graph

import (
	"errors"
	"testing"
	"time"

	"GopherPaper/pkg/errs"
)

func TestNetworkRebuildJobIsOwnerScoped(t *testing.T) {
	job := &rebuildJob{
		ID:        "job-1",
		Status:    rebuildJobRunning,
		Errors:    []rebuildJobError{},
		StartedAt: time.Now(),
		owner:     "student-a",
	}
	networkRebuildJobs.Lock()
	previousJobs := networkRebuildJobs.byID
	previousActive := networkRebuildJobs.activeByOwner
	networkRebuildJobs.byID = map[string]*rebuildJob{job.ID: job}
	networkRebuildJobs.activeByOwner = map[string]string{"student-a": job.ID}
	networkRebuildJobs.Unlock()
	t.Cleanup(func() {
		networkRebuildJobs.Lock()
		networkRebuildJobs.byID = previousJobs
		networkRebuildJobs.activeByOwner = previousActive
		networkRebuildJobs.Unlock()
	})

	result, ok := networkRebuildJob("student-a", job.ID)
	if !ok {
		t.Fatal("owner should be able to read its rebuild job")
	}
	if result.Errors == nil {
		t.Fatal("empty rebuild errors must serialize as [] instead of null")
	}
	if _, ok := networkRebuildJob("student-b", job.ID); ok {
		t.Fatal("another owner must not be able to read the rebuild job")
	}
}

func TestRebuildFailureMessageDoesNotExposeInternalError(t *testing.T) {
	if got := rebuildFailureMessage("metadata", errs.ErrPaperNotFound); got != "论文记录不存在" {
		t.Fatalf("unexpected not-found message: %q", got)
	}
	if got := rebuildFailureMessage("relations", errors.New("database password leaked")); got != "语义关系更新失败" {
		t.Fatalf("unexpected sanitized relation message: %q", got)
	}
}
