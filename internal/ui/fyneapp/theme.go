package fyneapp

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type androidTheme struct{}

func (androidTheme) Color(c fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(c, v)
}

func (androidTheme) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (androidTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (androidTheme) Size(s fyne.ThemeSizeName) float32 {
	switch s {
	case theme.SizeNameText:
		return 15
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameScrollBar:
		return 8
	default:
		return theme.DefaultTheme().Size(s)
	}
}
