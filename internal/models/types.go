package models

// View represents the current active view in the UI.
type View int

const (
	ViewContainers View = iota
	ViewImages
	ViewVolumes
	ViewNetworks
	ViewSettings
)

// String returns the display name of the view.
func (v View) String() string {
	switch v {
	case ViewContainers:
		return "Containers"
	case ViewImages:
		return "Images"
	case ViewVolumes:
		return "Volumes"
	case ViewNetworks:
		return "Networks"
	case ViewSettings:
		return "Settings"
	default:
		return "Unknown"
	}
}

// ContainerState represents the state of a container for UI purposes.
type ContainerState int

const (
	StateUnknown ContainerState = iota
	StateRunning
	StateStopped
	StatePaused
	StateRestarting
	StateCreated
	StateRemoving
)

// ParseContainerState converts a Docker state string to ContainerState.
func ParseContainerState(state string) ContainerState {
	switch state {
	case "running":
		return StateRunning
	case "exited", "dead":
		return StateStopped
	case "paused":
		return StatePaused
	case "restarting":
		return StateRestarting
	case "created":
		return StateCreated
	case "removing":
		return StateRemoving
	default:
		return StateUnknown
	}
}
