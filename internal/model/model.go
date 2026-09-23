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


type PlanReport struct {
	Repository              string         `json:"repository"`
	Commits                 int            `json:"commits_scanned"`
	MatchedCommits          int            `json:"matched_commits"`
	CommitMatchPct          float64        `json:"commit_match_pct"`
	Matches                 []Match        `json:"matches"`
	RuleMatches             map[string]int `json:"rule_matches"`
	CommitsToRewrite        int            `json:"commits_to_rewrite"`
	DescendantCommits       int            `json:"descendant_commits_to_rewrite"`
	ParentLinksToRewrite    int            `json:"parent_links_to_rewrite"`
	BranchesToMove          int            `json:"branches_to_move"`
	TagRefsToMove           int            `json:"tag_refs_to_move"`
	RefsToMove              int            `json:"refs_to_move"`
	AnnotatedTagsToRewrite  int            `json:"annotated_tags_to_rewrite"`
	SignaturesAtRisk        int            `json:"signatures_at_risk"`
	ObjectWritesEstimate    int            `json:"object_writes_estimate"`
	RewriteReady            bool           `json:"rewrite_ready"`
	RewriteBlocker          string         `json:"rewrite_blocker,omitempty"`
	AffectedRefs            []string       `json:"affected_refs"`
	DurationMS              int64          `json:"duration_ms"`
}
