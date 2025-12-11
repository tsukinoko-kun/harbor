package ui

import (
	"context"
	"image"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/tsukinoko-kun/harbor/internal/docker"
	"github.com/tsukinoko-kun/harbor/internal/models"
)

// App represents the main application.
type App struct {
	window *app.Window
	theme  *Theme
	docker *docker.Client

	// UI State
	currentView models.View
	sidebar     *Sidebar
	containers  *ContainersView
	images      *ImagesView
	volumes     *VolumesView
	networks    *NetworksView

	// Data
	mu              sync.RWMutex
	containerGroups []docker.ContainerGroup
	imageList       []docker.Image
	volumeList      []docker.Volume
	networkList     []docker.Network
	lastError       error
}

// NewApp creates a new application instance.
func NewApp(dockerClient *docker.Client) *App {
	theme := NewTheme()

	a := &App{
		window:      nil, // Set during Run
		theme:       theme,
		docker:      dockerClient,
		currentView: models.ViewContainers,
	}

	a.sidebar = NewSidebar(theme, a.onViewChange)
	a.containers = NewContainersView(theme)
	a.images = NewImagesView(theme)
	a.volumes = NewVolumesView(theme)
	a.networks = NewNetworksView(theme)

	return a
}

// Run starts the application event loop.
func (a *App) Run() error {
	a.window = new(app.Window)
	a.window.Option(
		app.Title("Harbor"),
		app.Size(unit.Dp(1200), unit.Dp(800)),
		app.MinSize(unit.Dp(800), unit.Dp(600)),
	)

	// Start data refresh goroutine
	go a.refreshLoop()

	// Run the event loop
	var ops op.Ops
	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) refreshLoop() {
	// Initial refresh
	a.refreshData()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		a.refreshData()
		if a.window != nil {
			a.window.Invalidate()
		}
	}
}

func (a *App) refreshData() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Refresh based on current view to prioritize visible data
	switch a.currentView {
	case models.ViewContainers:
		if groups, err := a.docker.ListContainersGrouped(ctx); err == nil {
			a.containerGroups = groups
		} else {
			a.lastError = err
		}
	case models.ViewImages:
		if images, err := a.docker.ListImages(ctx); err == nil {
			a.imageList = images
		} else {
			a.lastError = err
		}
	case models.ViewVolumes:
		if volumes, err := a.docker.ListVolumes(ctx); err == nil {
			a.volumeList = volumes
		} else {
			a.lastError = err
		}
	case models.ViewNetworks:
		if networks, err := a.docker.ListNetworks(ctx); err == nil {
			a.networkList = networks
		} else {
			a.lastError = err
		}
	}
}

func (a *App) onViewChange(view models.View) {
	if a.currentView != view {
		a.currentView = view
		// Trigger immediate refresh for new view
		go a.refreshData()
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	// Fill background
	paint.FillShape(gtx.Ops, a.theme.Colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// Sidebar
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(unit.Dp(200))
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(200))
			return a.layoutSidebar(gtx)
		}),
		// Divider
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutDivider(gtx)
		}),
		// Main content
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutContent(gtx)
		}),
	)
}

func (a *App) layoutSidebar(gtx layout.Context) layout.Dimensions {
	// Fill sidebar background
	paint.FillShape(gtx.Ops, a.theme.Colors.SidebarBg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return a.sidebar.Layout(gtx, a.currentView)
}

func (a *App) layoutDivider(gtx layout.Context) layout.Dimensions {
	width := gtx.Dp(unit.Dp(1))
	rect := clip.Rect{Max: image.Point{X: width, Y: gtx.Constraints.Max.Y}}
	paint.FillShape(gtx.Ops, a.theme.Colors.Border, rect.Op())
	return layout.Dimensions{Size: image.Point{X: width, Y: gtx.Constraints.Max.Y}}
}

func (a *App) layoutContent(gtx layout.Context) layout.Dimensions {
	// Fill content background
	paint.FillShape(gtx.Ops, a.theme.Colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())

	a.mu.RLock()
	defer a.mu.RUnlock()

	switch a.currentView {
	case models.ViewContainers:
		return a.containers.Layout(gtx, a.containerGroups)
	case models.ViewImages:
		return a.images.Layout(gtx, a.imageList)
	case models.ViewVolumes:
		return a.volumes.Layout(gtx, a.volumeList)
	case models.ViewNetworks:
		return a.networks.Layout(gtx, a.networkList)
	default:
		return layout.Dimensions{}
	}
}
