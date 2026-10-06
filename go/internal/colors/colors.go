// Package colors provides tview-style color tag helpers mirroring the
// Python colors.py module (which used ANSI escapes via the `colored` lib).
package colors

import "fmt"

// C wraps text in a tview dynamic-color tag, optionally bold, resetting
// afterwards. An empty color leaves the text unstyled.
func C(text string, color string, bold bool) string {
	if color == "" {
		return text
	}
	style := ""
	if bold {
		style = "b"
	}
	if style == "" {
		return fmt.Sprintf("[%s]%s[-]", color, text)
	}
	return fmt.Sprintf("[%s::%s]%s[-:-:-]", color, style, text)
}

func StatusOK(text string) string {
	return C(text, "green", true)
}

func StatusWarn(text string) string {
	return C(text, "yellow", true)
}

func StatusErr(text string) string {
	return C(text, "red", true)
}
