package history

type Entry struct {
	Connection string `json:"connection"`
	Query      string `json:"query"`
}
