package service

import "context"

type JevRequest struct {
	JobID string
	Input map[string]string
}

type JevDecision struct {
	Configured  bool
	Decision    string
	Confidence  float64
	RuleVersion string
}

type JevClient interface {
	Decide(context.Context, JevRequest) (JevDecision, error)
}

type noopJevClient struct{}

func NewJevClient(_ string) JevClient {
	return noopJevClient{}
}

func (noopJevClient) Decide(context.Context, JevRequest) (JevDecision, error) {
	return JevDecision{Configured: false}, nil
}
