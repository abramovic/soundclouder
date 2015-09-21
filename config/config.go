package config

type Configuration struct {
	Host       string `json:"host"`
	ClientId   string `json:"client_id"`
	MaxWorkers int    `json:"max_workers"`
}
