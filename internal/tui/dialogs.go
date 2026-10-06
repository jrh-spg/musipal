package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const dialogPageName = "dialog"

// centered wraps p in a Flex that centers it with a fixed width/height.
func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}

// showInputDialog displays a modal text input with OK/Cancel buttons.
// onDone is called with the entered text and whether OK was pressed.
func (a *App) showInputDialog(title, label, initial string, onDone func(value string, ok bool)) {
	form := tview.NewForm()
	form.AddInputField(label, initial, 40, nil, nil)
	finish := func(ok bool) {
		value := form.GetFormItem(0).(*tview.InputField).GetText()
		a.pages.RemovePage(dialogPageName)
		a.tv.SetFocus(a.focusedView())
		onDone(value, ok)
	}
	form.AddButton("OK", func() { finish(true) })
	form.AddButton("Cancel", func() { finish(false) })
	form.SetBorder(true).SetTitle(" " + title + " ")
	form.SetCancelFunc(func() { finish(false) })

	a.pages.AddPage(dialogPageName, centered(form, 60, 7), true, true)
	a.tv.SetFocus(form)
}

// showConfirmDialog displays a modal Yes/No confirmation.
func (a *App) showConfirmDialog(title, text string, onDone func(confirmed bool)) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(_ int, label string) {
			a.pages.RemovePage(dialogPageName)
			a.tv.SetFocus(a.focusedView())
			onDone(label == "Yes")
		})
	modal.SetBorder(true).SetTitle(" " + title + " ")
	a.pages.AddPage(dialogPageName, modal, true, true)
	a.tv.SetFocus(modal)
}

// showMessageDialog displays a modal message with a single OK button.
func (a *App) showMessageDialog(title, text string) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(_ int, _ string) {
			a.pages.RemovePage(dialogPageName)
			a.tv.SetFocus(a.focusedView())
		})
	modal.SetBorder(true).SetTitle(" " + title + " ")
	modal.SetBackgroundColor(tcell.ColorDefault)
	a.pages.AddPage(dialogPageName, modal, true, true)
	a.tv.SetFocus(modal)
}

func (a *App) focusedView() tview.Primitive {
	if a.currentFocus() == "queue" {
		return a.queueView
	}
	return a.libraryView
}
