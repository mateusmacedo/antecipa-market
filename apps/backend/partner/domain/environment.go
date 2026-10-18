package domain

type Environment string

const (
	EnvironmentDevelopment  Environment = "development"
	EnvironmentIntegration  Environment = "integration"
	EnvironmentHomologation Environment = "homologation"
	EnvironmentProduction   Environment = "production"
)
