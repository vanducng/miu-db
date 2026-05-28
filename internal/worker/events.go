package worker

type Event struct {
	Name   string `json:"event"`
	CallID string `json:"call_id"`
	Seq    int    `json:"seq"`
}
