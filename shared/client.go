package shared

import "sync"

type Client interface {
	CreateFunction(wg *sync.WaitGroup, cfg Config, d Deployment)
	UpdateFunction(wg *sync.WaitGroup, cfg Config, d Deployment)
	ListFunctions(cfg Config) []string
}

//Wrapper for config for different cloud providers
type Config struct {
}
