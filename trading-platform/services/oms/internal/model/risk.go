package model

type CheckResult struct {
	Passed  bool                   `json:"passed"`
	Check   string                 `json:"check"`
	Reason  string                 `json:"reason"`
	Details map[string]interface{} `json:"details,omitempty"`
}
