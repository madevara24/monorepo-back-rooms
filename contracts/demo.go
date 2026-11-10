package contracts

// ResetCounts is the agent's report after a bulk demo reset.
type ResetCounts struct {
	Containers int `json:"containers_deleted"`
	Volumes    int `json:"volumes_deleted"`
	Orphans    int `json:"orphans_deleted"`
}
