package kubby

import "fmt"

type ExistingKubeClusterError struct {
	name string
}

func (err *ExistingKubeClusterError) Error() string {
	return fmt.Sprintf("error: cluster %s already exists", err.name)
}

type ExistingKubeConfigError struct {
	path string
}

func (err *ExistingKubeConfigError) Error() string {
	return fmt.Sprintf("error: kubeconfig at %s already exists", err.path)
}

type ExceededMaxAttemptError struct {
	attempts int
}

func (err *ExceededMaxAttemptError) Error() string {
	return fmt.Sprintf("error: exceeded max attempts (%v)", err.attempts)
}

type MissingFieldError struct {
	field string
}

func (err *MissingFieldError) Error() string {
	return fmt.Sprintf("error: field %s is missing but it is required", err.field)
}

type BadContainerNameError struct {
	name string
}

func (err *BadContainerNameError) Error() string {
	return fmt.Sprintf("error: no such container named %s", err.name)
}

type BadImageBuildError struct {
	output string
}

func (err *BadImageBuildError) Error() string {
	return fmt.Sprintf("failed building image: %s", err.output)
}

type FailedJobError struct {
	name string
}

func (err *FailedJobError) Error() string {
	return fmt.Sprintf("job %s failed", err.name)
}

type BadPodNameError struct {
	name string
}

func (err *BadPodNameError) Error() string {
	return fmt.Sprintf("no pod named %s exists", err.name)
}
