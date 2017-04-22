package ui

import (
	"github.com/gdamore/tcell"
	"github.com/gdamore/tcell/views"
)

type tuiWindow struct {
	shutdownch chan bool

	app *views.Application

	containers *tablePanel
	pools      *tablePanel

	views.Panel
}

func (a *tuiWindow) HandleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'Q', 'q':
				a.shutdownch <- true
				return true
			}
		}
	}
	return a.Panel.HandleEvent(ev)
}

func (a *tuiWindow) Draw() {
	a.Panel.Draw()
}

func (a *tuiWindow) updateContainers(upsert map[string]tuiTR, remove map[string]tuiTR) bool {
	resize := a.containers.table.handleUpdates(upsert, remove)
	if resize {
		a.containers.Resize()
	}
	return resize
}

func (a *tuiWindow) updatePools(upsert map[string]tuiTR, remove map[string]tuiTR) bool {
	resize := a.pools.table.handleUpdates(upsert, remove)
	if resize {
		a.pools.Resize()
	}
	return resize
}

func newTableWidget(name string) *views.Panel {
	panel := views.NewPanel()

	title := views.NewTextBar()
	title.SetStyle(tcell.StyleDefault.
		Background(tcell.ColorTeal).
		Foreground(tcell.ColorWhite))

	title.SetCenter(name, tcell.StyleDefault)

	panel.SetTitle(title)

	return panel
}

type tablePanel struct {
	table *tuiTable
	views.Panel
}

func containersWidget() *tablePanel {
	maxLifecycleLen := len("healthcheck")
	maxActionLen := len("postgres.truncate")

	tw := newTableWidget("Containers")
	t := newTUITable([]tuiTH{
		{"Pool", 15},
		{"ID", 12},
		{"State", cstateMaxLen},
		{"Lifecycle", maxLifecycleLen},
		{"Action", maxActionLen},
		{"Attempt", 5},
		{"Error", 0},
	})

	tw.SetContent(t)
	tw.Resize()

	return &tablePanel{
		table: t,
		Panel: *tw,
	}
}

func poolsWidget() *tablePanel {
	tw := newTableWidget("Pools")
	t := newTUITable([]tuiTH{
		{"Name", 15},
		{"State", pstateMaxLen},
		{"Ready", 3},
		{"Total", 3},
		{"Pending", 3},
		{"Error", 0},
	})
	tw.SetContent(t)
	tw.Resize()
	return &tablePanel{
		table: t,
		Panel: *tw,
	}
}

func createTuiApp(shutdownch chan bool) (*views.Application, *tuiWindow) {
	title := views.NewTextBar()
	title.SetStyle(tcell.StyleDefault.
		Background(tcell.ColorTeal).
		Foreground(tcell.ColorWhite))
	title.SetCenter("Ephemerald", tcell.StyleDefault)

	keybar := views.NewSimpleStyledText()

	keybar.RegisterStyle('N', tcell.StyleDefault.
		Background(tcell.ColorSilver).
		Foreground(tcell.ColorBlack))

	keybar.RegisterStyle('A', tcell.StyleDefault.
		Background(tcell.ColorSilver).
		Foreground(tcell.ColorRed))

	keybar.SetMarkup("[%AQ%N] Quit")

	app := &views.Application{}

	window := &tuiWindow{
		shutdownch: shutdownch,
		app:        app,
		containers: containersWidget(),
		pools:      poolsWidget(),
	}

	layout := views.NewBoxLayout(views.Vertical)

	layout.AddWidget(window.containers, 0.0)
	layout.AddWidget(window.pools, 0.0)
	layout.AddWidget(views.NewSpacer(), 1.0)

	window.SetMenu(keybar)
	window.SetContent(layout)

	app.SetStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorBlack))

	app.SetRootWidget(window)

	return app, window
}
