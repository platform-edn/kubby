package kubby

type ClusterStatus int

const (
	Alive ClusterStatus = iota
	Dead
)

func (status ClusterStatus) String() string {
	types := []string{
		"Alive",
		"Dead",
	}

	return types[int(status)]
}
