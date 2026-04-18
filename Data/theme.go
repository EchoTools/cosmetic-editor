package data

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type ModernTheme struct{}

var _ fyne.Theme = (*ModernTheme)(nil)

func (m *ModernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.RGBA{R: 30, G: 136, B: 229, A: 255} // Pulse Blue
	}
	if name == theme.ColorNameBackground {
		return color.RGBA{R: 18, G: 18, B: 18, A: 255} // Deep Dark
	}
	if name == theme.ColorNameInputBackground {
		return color.RGBA{R: 28, G: 28, B: 28, A: 255}
	}
	if name == theme.ColorNameSeparator {
		return color.RGBA{R: 44, G: 44, B: 44, A: 255}
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m *ModernTheme) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (m *ModernTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (m *ModernTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNamePadding {
		return 8
	}
	if n == theme.SizeNameInnerPadding {
		return 4
	}
	return theme.DefaultTheme().Size(n)
}
