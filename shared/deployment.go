package shared

type Deployment struct {
	Archive         string
	Name            string
	MemorySize      int32
	Timeout         int32
	Runtime         string
	Provider        ProviderName
	HandlerFile     string
	HandlerFunction string
	Region          string
	Bucket          string
	Key             string
}

type DeploymentDto struct {
	Archive    string     `mapstructure:"archive"`
	Name       string     `mapstructure:"name"`
	MemorySize int32      `mapstructure:"memory"`
	Timeout    int32      `mapstructure:"timeout"`
	Providers  []Provider `mapstructure:"providers"`
}

type Provider struct {
	Name    ProviderName `mapstructure:"name"`
	Handler string       `mapstructure:"handler"`
	Regions []string     `mapstructure:"regions"`
	Runtime string       `mapstructure:"runtime"`
}

type ProviderName string

const (
	ProviderAWS    ProviderName = "AWS"
	ProviderGoogle ProviderName = "Google"
)

func CheckDeployment(de Deployment) error {
	var unparsedKeys []string

	if len(de.Archive) == 0 {
		unparsedKeys = append(unparsedKeys, "Archive")
	}
	if len(de.Name) == 0 {
		unparsedKeys = append(unparsedKeys, "Name")
	}
	if de.MemorySize <= 0 {
		unparsedKeys = append(unparsedKeys, "MemorySize")
	}
	if len(de.Runtime) == 0 {
		unparsedKeys = append(unparsedKeys, "Runtime")
	}
	if len(de.Provider) == 0 || !(string(de.Provider) == string(ProviderAWS) || string(de.Provider) == string(ProviderGoogle)) {
		unparsedKeys = append(unparsedKeys, "Provider")
	}
	if len(de.HandlerFile) == 0 {
		unparsedKeys = append(unparsedKeys, "HandlerFile")
	}
	if len(de.HandlerFunction) == 0 {
		unparsedKeys = append(unparsedKeys, "HandlerFunction")
	}
	if len(de.Region) == 0 {
		unparsedKeys = append(unparsedKeys, "Regions")
	}

	if len(unparsedKeys) == 0 {
		return nil
	} else {
		return &DeploymentParseError{UnparsedKeys: unparsedKeys}
	}
}
