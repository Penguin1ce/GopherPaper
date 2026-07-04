package graph

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"GopherPaper/pkg/errs"
)

const (
	rebuildJobQueued    = "queued"
	rebuildJobRunning   = "running"
	rebuildJobCompleted = "completed"
	rebuildJobFailed    = "failed"
	rebuildJobRetention = time.Hour
)

type rebuildProgressFunc func(total int, paperID, stage string, err error)

type rebuildJobError struct {
	PaperID string `json:"paper_id,omitempty"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type rebuildJob struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	Total      int               `json:"total"`
	Completed  int               `json:"completed"`
	Succeeded  int               `json:"succeeded"`
	Failed     int               `json:"failed"`
	WorkTotal  int               `json:"work_total"`
	WorkDone   int               `json:"work_done"`
	Phase      string            `json:"phase,omitempty"`
	CurrentID  string            `json:"current_paper_id,omitempty"`
	Errors     []rebuildJobError `json:"errors"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`

	owner string
}

var networkRebuildJobs = struct {
	sync.RWMutex
	byID          map[string]*rebuildJob
	activeByOwner map[string]string
}{
	byID:          map[string]*rebuildJob{},
	activeByOwner: map[string]string{},
}

func startNetworkRebuildJob(owner string) (rebuildJob, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return rebuildJob{}, errors.New("graph rebuild: empty owner")
	}

	networkRebuildJobs.Lock()
	pruneNetworkRebuildJobsLocked(time.Now())
	if id := networkRebuildJobs.activeByOwner[owner]; id != "" {
		if current := networkRebuildJobs.byID[id]; current != nil {
			job := cloneRebuildJob(current)
			networkRebuildJobs.Unlock()
			return job, nil
		}
		delete(networkRebuildJobs.activeByOwner, owner)
	}
	job := &rebuildJob{
		ID:        uuid.NewString(),
		Status:    rebuildJobQueued,
		Errors:    []rebuildJobError{},
		StartedAt: time.Now(),
		owner:     owner,
	}
	networkRebuildJobs.byID[job.ID] = job
	networkRebuildJobs.activeByOwner[owner] = job.ID
	out := cloneRebuildJob(job)
	networkRebuildJobs.Unlock()

	go runNetworkRebuildJob(job.ID, owner)
	return out, nil
}

func networkRebuildJob(owner, id string) (rebuildJob, bool) {
	networkRebuildJobs.RLock()
	defer networkRebuildJobs.RUnlock()
	job := networkRebuildJobs.byID[id]
	if job == nil || job.owner != owner {
		return rebuildJob{}, false
	}
	return cloneRebuildJob(job), true
}

func runNetworkRebuildJob(id, owner string) {
	updateNetworkRebuildJob(id, func(job *rebuildJob) {
		job.Status = rebuildJobRunning
	})

	err := rebuildOwnerSemanticGraphsFromMeta(owner, func(total int, paperID, stage string, syncErr error) {
		updateNetworkRebuildJob(id, func(job *rebuildJob) {
			if stage == "total" {
				job.Total = total
				job.WorkTotal = total * 2
				return
			}
			job.CurrentID = paperID
			if stage == "metadata_prepared" {
				job.Phase = "metadata"
				job.WorkDone++
				return
			}
			if stage == "metadata" {
				job.Phase = "metadata"
				job.WorkDone += 2
			} else {
				job.Phase = "relations"
				job.WorkDone++
			}
			job.Completed++
			if syncErr == nil {
				job.Succeeded++
				return
			}
			job.Failed++
			job.Errors = append(job.Errors, rebuildJobError{
				PaperID: paperID,
				Stage:   stage,
				Message: rebuildFailureMessage(stage, syncErr),
			})
		})
	})

	finishedAt := time.Now()
	updateNetworkRebuildJob(id, func(job *rebuildJob) {
		job.FinishedAt = &finishedAt
		if err != nil {
			job.Status = rebuildJobFailed
			job.Errors = append(job.Errors, rebuildJobError{
				Stage:   "prepare",
				Message: "读取待同步论文失败",
			})
		} else {
			job.Status = rebuildJobCompleted
			job.WorkDone = job.WorkTotal
		}
	})

	networkRebuildJobs.Lock()
	if networkRebuildJobs.activeByOwner[owner] == id {
		delete(networkRebuildJobs.activeByOwner, owner)
	}
	networkRebuildJobs.Unlock()
}

func updateNetworkRebuildJob(id string, update func(*rebuildJob)) {
	networkRebuildJobs.Lock()
	defer networkRebuildJobs.Unlock()
	if job := networkRebuildJobs.byID[id]; job != nil {
		update(job)
	}
}

func cloneRebuildJob(job *rebuildJob) rebuildJob {
	out := *job
	out.Errors = make([]rebuildJobError, len(job.Errors))
	copy(out.Errors, job.Errors)
	return out
}

func pruneNetworkRebuildJobsLocked(now time.Time) {
	for id, job := range networkRebuildJobs.byID {
		if job.FinishedAt != nil && now.Sub(*job.FinishedAt) > rebuildJobRetention {
			delete(networkRebuildJobs.byID, id)
		}
	}
}

func rebuildFailureMessage(stage string, err error) string {
	switch {
	case errors.Is(err, errPaperMetaNotFound):
		return "论文元信息尚未生成"
	case errors.Is(err, errs.ErrPaperNotFound):
		return "论文记录不存在"
	case errors.Is(err, errs.ErrPaperForbidden):
		return "无权同步该论文"
	case stage == "relations":
		return "语义关系更新失败"
	default:
		return "论文实体同步失败"
	}
}
