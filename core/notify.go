package core

import "context"

//go:generate mockgen -package mock -destination ../mock/notify_mock.go . NotifyService,Notifier

// StatusState is a commit-status / verdict state.
type StatusState string

// Status states.
const (
	StatusPending StatusState = "pending"
	StatusSuccess StatusState = "success"
	StatusFailure StatusState = "failure"
	StatusError   StatusState = "error"
)

// Status is a commit status to post to an SCM.
type Status struct {
	State  StatusState
	Label  string // status context, e.g. "coverage/covergates"
	Desc   string // short human description
	Target string // URL the status links to (build detail page)
}

// Verdict is the evaluated outcome of a finalized build under the repo's policy.
type Verdict struct {
	State       StatusState
	Coverage    float64 // ratio 0..1
	Change      float64 // ratio delta vs base (negative = decrease)
	HasBase     bool
	Description string
}

// Notifier delivers a build's verdict over one channel (status, comment, email, ...).
type Notifier interface {
	Notify(ctx context.Context, repo *Repo, build *Build, verdict *Verdict) error
}

// NotifyService computes the verdict for a finalized build and dispatches notifiers.
type NotifyService interface {
	Notify(ctx context.Context, repo *Repo, build *Build) error
}
