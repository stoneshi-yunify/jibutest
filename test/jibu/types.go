package jibu

type PhaseType string

const (
	// PhaseReady indicates all conditions are met
	PhaseReady PhaseType = "Ready"
	// PhaseNotReady is used if reconcile fails
	PhaseNotReady PhaseType = "NotReady"
	// PhaseError is used when reconcile fails or there is any of false ready from conditions
	PhaseError PhaseType = "Error"
	// PhaseDeleted indicates backend resources are already reclaimed
	PhaseDeleted PhaseType = "PhaseDeleted"
	// PhaseDeleted indicates a deletion is requested
	PhaseDeleting PhaseType = "PhaseDeleting"

	// JobPhaseNotStarted means a job is not started yet
	JobPhaseNotStarted PhaseType = "JobNotStarted"
	// JobPhaseInProgress means a job is in progress
	JobPhaseInProgress PhaseType = "JobInProgress"
	// JobPhaseCompleted means a job has completed successfully
	JobPhaseCompleted PhaseType = "JobCompleted"
	// JobPhaseFailed means a job has failed
	JobPhaseFailed PhaseType = "JobFailed"
	// JobPahseCanceled means a job has been canceled
	JobPhaseCanceled PhaseType = "JobCanceled"
	// JobSubmitted means a job is submitted to backend controller
	JobPhaseSubmitted PhaseType = "JobSubmitted"
)
