package main

import (
	"context"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	DefaultRefreshInterval = 30 * time.Second
)

var (
	ctrlQ = &desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierControl}

	ctrlR = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}
	ctrlS = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}
	F5    = &desktop.CustomShortcut{KeyName: fyne.KeyF5}
)

type GuiConfig struct {
	title           string
	refreshInterval time.Duration
	overrideTheme   string

	LogPrintf func(string, ...interface{})
}

func DefaultGuiConfig() *GuiConfig {
	return &GuiConfig{
		title:           ProgName,
		refreshInterval: DefaultRefreshInterval,
		overrideTheme:   "",
		LogPrintf:       func(string, ...interface{}) {},
	}
}

type GuiState struct {
	*GuiConfig
	client *ProxmoxClient

	mu        sync.Mutex
	resources []Resource

	app        fyne.App
	mainWindow fyne.Window

	refresh      chan struct{}
	updateTicker *time.Ticker

	table      *MyTable
	errorLabel *widget.Label
}

func runGui(c *GuiConfig, client *ProxmoxClient) {
	state := &GuiState{
		GuiConfig:    c,
		client:       client,
		app:          app.NewWithID(ProgName),
		refresh:      make(chan struct{}),
		updateTicker: time.NewTicker(c.refreshInterval),
	}
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	switch state.overrideTheme {
	case "dark":
		state.app.Settings().SetTheme(theme.DarkTheme())
	case "light":
		state.app.Settings().SetTheme(theme.LightTheme())
	}

	state.mainWindow = state.createMainWindow()
	state.mainWindow.Show()

	go state.loadResourcesLoop(ctx)

	state.app.Run()
}

func (s *GuiState) createMainWindow() fyne.Window {
	main := s.app.NewWindow(ProgName)
	main.Resize(fyne.Size{Width: 600, Height: 480})

	main.Canvas().AddShortcut(ctrlQ, func(_ fyne.Shortcut) { s.app.Quit() })
	main.Canvas().AddShortcut(ctrlS, func(_ fyne.Shortcut) { s.withCurrentVM(s.startStopVM) })
	main.Canvas().AddShortcut(ctrlR, func(_ fyne.Shortcut) { s.withCurrentVM(s.client.Reset) })

	s.table = s.createTable()
	s.table.OnActivated = func() { s.withCurrentVM(s.showVM) }
	s.table.OnTyped = func(event *fyne.KeyEvent) {
		switch event.Name {
		case F5.KeyName:
			s.triggerRefresh()
		}
	}

	title := canvas.NewText(s.title,
		s.app.Settings().Theme().Color(theme.ColorNameForeground, s.app.Settings().ThemeVariant()))
	title.TextSize *= 2
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter

	s.errorLabel = widget.NewLabel("")
	s.errorLabel.Importance = widget.DangerImportance
	s.errorLabel.Hide()

	infoLabel := widget.NewLabel("Enter: open | F5: refresh | Ctrl-Q: quit | Ctrl-S: start/stop | Ctrl-R: reset")

	infoRow := container.NewVBox(s.errorLabel, infoLabel)

	cont := container.NewBorder(title, infoRow, nil, nil, s.table)
	main.SetContent(cont)

	main.Canvas().Focus(s.table)
	main.SetMaster()
	main.CenterOnScreen()
	return main
}

func (s *GuiState) showVM(vm *Resource) error {
	s.mainWindow.Hide()
	s.updateTicker.Stop()
	defer s.mainWindow.Show()
	defer s.triggerRefresh()

	return s.client.SpiceProxy(vm)
}

func (s *GuiState) startStopVM(vm *Resource) error {
	var err error
	if vm.Status == "stopped" {
		err = s.client.Operate(vm, "start")
	} else if vm.Status == "running" {
		err = s.client.Operate(vm, "stop")
	}
	if err != nil {
		return err
	}
	s.triggerRefresh()
	return nil
}

func (s *GuiState) createTable() *MyTable {
	return NewMyTable(
		[]string{"VM Id", "Name", "Status"},
		[]float32{64, 256, 64},
		func() (rows int) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return len(s.resources)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("table cell")
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			s.mu.Lock()
			defer s.mu.Unlock()
			vm := s.resources[id.Row]

			label := object.(*widget.Label)

			switch id.Col {
			case 0:
				label.SetText(strconv.Itoa(vm.VmId))
				label.Alignment = fyne.TextAlignTrailing
			case 1:
				label.SetText(vm.Name)
				label.Alignment = fyne.TextAlignLeading
			case 2:
				label.SetText(vm.Status)
				label.Alignment = fyne.TextAlignLeading
			}
		},
	)
}

func (s *GuiState) error(err error) {
	if err != nil {
		s.LogPrintf("Error: %s", err)
		s.errorLabel.SetText("Error: " + err.Error())
		s.errorLabel.Show()
		s.errorLabel.Refresh()
	} else {
		// s.errorLabel.SetText("")
		// s.errorLabel.Hide()
	}
}

func (s *GuiState) loadResourcesLoop(ctx context.Context) {
	for {
		s.error(s.loadResources())

		select {
		case <-s.updateTicker.C:
		case <-s.refresh:
			s.updateTicker.Reset(s.refreshInterval) // reset timer to fire after interval
		case <-ctx.Done():
			return
		}
	}
}

func (s *GuiState) loadResources() error {
	s.LogPrintf("Loading resources...")
	resources, err := s.client.Resources()
	if err != nil {
		s.error(err)
	}

	vms := filter(resources, func(r Resource) bool { return r.Type == GuestType })
	s.LogPrintf("%d VMs found", len(vms))

	// Load status from the cluster node. The cluster takes a few seconds to update.
	for i := range vms {
		status, err := s.client.Status(&vms[i])
		if err != nil {
			continue
		}
		vms[i].Status = status
	}

	s.mu.Lock()
	s.resources = vms
	s.mu.Unlock()

	s.table.Refresh()
	return nil
}

func (s *GuiState) triggerRefresh() {
	select {
	case s.refresh <- struct{}{}:
	default: // do not block on a refresh!
	}
}

func (s *GuiState) withCurrentVM(f func(vm *Resource) error) {
	s.mu.Lock()
	vm := &s.resources[s.table.CurrentRow]
	s.mu.Unlock()
	if vm != nil {
		err := f(vm)
		s.error(err)
	}
}
