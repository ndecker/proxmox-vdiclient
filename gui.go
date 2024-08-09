package main

import (
	"context"
	"log"
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

var (
	ctrlQ = &desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierControl}

	ctrlR = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}
	ctrlS = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}
)

type GuiState struct {
	client *ProxmoxClient
	config *Config

	mu        sync.Mutex
	resources []Resource

	refresh      chan struct{}
	updateTicker *time.Ticker

	table      *MyTable
	errorLabel *widget.Label
}

func (s *GuiState) withCurrentVM(f func(vm *Resource) error) func(shortcut fyne.Shortcut) {
	return func(_ fyne.Shortcut) {
		s.mu.Lock()
		vm := &s.resources[s.table.CurrentRow]
		s.mu.Unlock()
		if vm != nil {
			err := f(vm)
			s.error(err)
		}
	}
}

func runGui(c *Config, client *ProxmoxClient) error {
	state := &GuiState{
		config:       c,
		client:       client,
		refresh:      make(chan struct{}),
		updateTicker: time.NewTicker(c.refreshInterval),
	}
	ctx, cancelCtx := context.WithCancel(context.Background())

	a := app.NewWithID(ProgName)

	switch state.config.overrideTheme {
	case "dark":
		a.Settings().SetTheme(theme.DarkTheme())
	case "light":
		a.Settings().SetTheme(theme.LightTheme())
	}

	main := mainWindow(a, state)
	main.Show()

	go state.loadResources(ctx)

	a.Run()
	cancelCtx()
	return nil
}

func mainWindow(a fyne.App, state *GuiState) fyne.Window {
	main := a.NewWindow(ProgName)
	main.Resize(fyne.Size{Width: 600, Height: 480})

	main.Canvas().AddShortcut(ctrlQ, func(shortcut fyne.Shortcut) { a.Quit() })

	main.Canvas().AddShortcut(ctrlS, state.withCurrentVM(func(vm *Resource) error {
		var err error
		if vm.Status == "stopped" {
			err = state.client.Operate(vm, "start")
		} else if vm.Status == "running" {
			err = state.client.Operate(vm, "stop")
		}
		if err != nil {
			return err
		}
		state.triggerRefresh()
		return nil
	}))

	main.Canvas().AddShortcut(ctrlR, state.withCurrentVM(state.client.Reset))

	state.table = createTable(state)
	state.table.OnActivated = state.withCurrentVM(func(vm *Resource) error {
		main.Hide()
		state.updateTicker.Stop()

		defer main.Show()
		defer state.triggerRefresh()

		return state.client.SpiceProxy(vm)
	})

	title := canvas.NewText(ProgName,
		a.Settings().Theme().Color(theme.ColorNameForeground, a.Settings().ThemeVariant()))
	title.TextSize *= 2
	title.TextStyle.Bold = true

	state.errorLabel = widget.NewLabel("")
	state.errorLabel.Importance = widget.DangerImportance
	state.errorLabel.Hide()

	cont := container.NewBorder(title, state.errorLabel, nil, nil, state.table)
	main.SetContent(cont)

	main.Canvas().Focus(state.table)
	main.SetMaster()
	main.CenterOnScreen()
	return main
}

func createTable(state *GuiState) *MyTable {
	table := NewMyTable(
		func() (rows int, cols int) {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.resources), 3

		},
		func() fyne.CanvasObject {
			return widget.NewLabel("table cell")
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			state.mu.Lock()
			defer state.mu.Unlock()
			vm := state.resources[id.Row]

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

	table.AddHeader("VM Id", "Name", "Status")
	table.SetColWidths(64, 256, 64)

	return table
}

func (s *GuiState) error(err error) {
	if err != nil {
		log.Println(err)
		s.errorLabel.SetText("Error: " + err.Error())
		s.errorLabel.Show()
	} else {
		s.errorLabel.SetText("")
		s.errorLabel.Hide()
	}
}

func (s *GuiState) loadResources(ctx context.Context) {
	for {
		log.Println("Loading resources...")
		resources, err := s.client.Resources()
		if err != nil {
			s.error(err)
			return
		}

		vms := filter(resources, func(r Resource) bool { return r.Type == GuestType })
		log.Printf("%d VMs found", len(vms))

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

		select {
		case <-ctx.Done():
			return
		case <-s.updateTicker.C:
		case <-s.refresh:
			s.updateTicker.Reset(s.config.refreshInterval) // reset timer to fire after interval
		}
	}
}

func (s *GuiState) triggerRefresh() { s.refresh <- struct{}{} }
