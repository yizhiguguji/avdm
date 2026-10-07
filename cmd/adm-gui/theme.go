package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

type admTheme struct {
	base fyne.Theme
}

var (
	admColorAppBG      = color.NRGBA{R: 9, G: 14, B: 25, A: 255}
	admColorPanelBG    = color.NRGBA{R: 15, G: 23, B: 42, A: 255}
	admColorPanelBG2   = color.NRGBA{R: 17, G: 24, B: 39, A: 255}
	admColorBorder     = color.NRGBA{R: 51, G: 65, B: 85, A: 255}
	admColorText       = color.NRGBA{R: 226, G: 232, B: 240, A: 255}
	admColorMuted      = color.NRGBA{R: 148, G: 163, B: 184, A: 255}
	admColorPrimary    = color.NRGBA{R: 37, G: 99, B: 235, A: 255}
	admColorDanger     = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
	admColorSuccess    = color.NRGBA{R: 16, G: 185, B: 129, A: 255}
	admColorPreviewBG  = color.NRGBA{R: 3, G: 7, B: 18, A: 255}
	admColorPreviewAlt = color.NRGBA{R: 8, G: 13, B: 24, A: 255}
)

func (t admTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground, theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return admColorAppBG
	case theme.ColorNameButton:
		return color.NRGBA{R: 34, G: 50, B: 72, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 22, G: 32, B: 48, A: 255}
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return admColorMuted
	case theme.ColorNameError:
		return admColorDanger
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError:
		return admColorText
	case theme.ColorNameFocus, theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return admColorPrimary
	case theme.ColorNameHover, theme.ColorNamePressed:
		return color.NRGBA{R: 51, G: 65, B: 85, A: 210}
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 11, G: 20, B: 36, A: 255}
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return color.NRGBA{R: 82, G: 97, B: 122, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 100, G: 116, B: 139, A: 220}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 15, G: 23, B: 42, A: 120}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 37, G: 99, B: 235, A: 120}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 90}
	case theme.ColorNameSuccess:
		return admColorSuccess
	case theme.ColorNameWarning:
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255}
	default:
		return t.base.Color(name, theme.VariantDark)
	}
}

func (t admTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t admTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t admTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameText:
		return 14
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.base.Size(name)
	}
}
