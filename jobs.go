package main

import (
	"strconv"
	"sync"
	"time"
)

type jobStatus uint8

const (
	jobStatusRunning jobStatus = iota
	jobStatusDone
	jobStatusError
)

const terminalJobTTL = 5 * time.Minute

type job struct {
	status     jobStatus
	progress   int
	message    string
	err        string
	terminalAt time.Time
}

var (
	jobsMu sync.Mutex
	jobs   = map[int64]*job{}
	nextID int64 = 1
)

func newJob() int64 {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	evictTerminalJobsLocked(time.Now())
	id := nextID
	nextID++
	jobs[id] = &job{status: jobStatusRunning, message: "starting"}
	return id
}

func getJob(id int64) *job {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	return jobs[id]
}

func setJobProgress(id int64, progress int, message string) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return
	}
	j.progress = progress
	if message != "" {
		j.message = message
	}
}

func evictTerminalJobsLocked(now time.Time) {
	cutoff := now.Add(-terminalJobTTL)
	for id, j := range jobs {
		if j.status == jobStatusRunning {
			continue
		}
		if !j.terminalAt.IsZero() && j.terminalAt.Before(cutoff) {
			delete(jobs, id)
		}
	}
}

func setJobDone(id int64, message string) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return
	}
	now := time.Now()
	j.status = jobStatusDone
	j.progress = 100
	j.message = message
	j.terminalAt = now
	evictTerminalJobsLocked(now)
}

func setJobError(id int64, err error) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return
	}
	now := time.Now()
	j.status = jobStatusError
	j.err = err.Error()
	j.message = j.err
	j.progress = 100
	j.terminalAt = now
	evictTerminalJobsLocked(now)
}

func jobProgress(id int64, scale int) int {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return 0
	}
	if j.status == jobStatusDone {
		return scale
	}
	// Terminal error reports full scale so the host can stop polling;
	// progressString carries the failure message (not "done").
	if j.status == jobStatusError {
		return scale
	}
	p := j.progress
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return (p * scale) / 100
}

func jobProgressString(id int64) string {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j, ok := jobs[id]
	if !ok {
		return "unknown job " + strconv.FormatInt(id, 10)
	}
	if j.err != "" {
		return j.err
	}
	return j.message
}
