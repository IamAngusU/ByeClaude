package model

type Match struct {
	Commit           string   `json:"commit"`
	Author           string   `json:"author"`
	Email            string   `json:"email"`
	AttributionName  string   `json:"attribution_name"`
	AttributionEmail string   `json:"attribution_email"`
	Rules            []string `json:"rules"`
	Line             string   `json:"line"`
}

type ScanReport struct {
	Repository     string         `json:"repository"`
	Commits        int            `json:"commits_scanned"`
	MatchedCommits int            `json:"matched_commits"`
	CommitMatchPct float64        `json:"commit_match_pct"`
	Matches        []Match        `json:"matches"`
	RuleMatches    map[string]int `json:"rule_matches"`
	DurationMS     int64          `json:"duration_ms"`
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
