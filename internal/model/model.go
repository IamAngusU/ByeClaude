package model

type Match struct {
	Commit string `json:"commit"`
	Author string `json:"author"`
	Email  string `json:"email"`
	Line   string `json:"line"`
}

type ScanReport struct {
	Repository string  `json:"repository"`
	Commits    int     `json:"commits_scanned"`
	Matches    []Match `json:"matches"`
	DurationMS int64   `json:"duration_ms"`
}

type RewriteReport struct {
	Repository        string `json:"repository"`
	Backup            string `json:"backup"`
	CommitsVisited    int    `json:"commits_visited"`
	CommitsRewritten  int    `json:"commits_rewritten"`
	RefsUpdated       int    `json:"refs_updated"`
	TagsRewritten     int    `json:"tags_rewritten"`
	SignaturesDropped int    `json:"signatures_dropped"`
	DurationMS        int64  `json:"duration_ms"`
}
