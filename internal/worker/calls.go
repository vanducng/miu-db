package worker

type CallStatus string

const (
	CallStarted   CallStatus = "started"
	CallDone      CallStatus = "done"
	CallCancelled CallStatus = "cancelled"
	CallError     CallStatus = "error"
)
