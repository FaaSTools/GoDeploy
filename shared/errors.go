package shared

import "fmt"

type RuntimeParseError struct {
	ProposedRuntime string
}

func (m *RuntimeParseError) Error() string {
	return "given runtime " + m.ProposedRuntime + " could not be parsed"
}

type DeploymentParseError struct {
	UnparsedKeys []string
}

func (m *DeploymentParseError) Error() string {
	return fmt.Sprintf("unable to parse keys of deployment file, %v", m.UnparsedKeys)
}
