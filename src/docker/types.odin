package docker

ContainerSummary :: struct {
	Id:     string,
	Names:  []string,
	Image:  string,
	State:  string,
	Status: string,
	Labels: map[string]string,
}
