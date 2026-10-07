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
	admColorAppBG      = color.NRGBA{R: 244, G: 246, B: 248, A: 255}
	admColorPanelBG    = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	admColorPanelBG2   = color.NRGBA{R: 248, G: 249, B: 251, A: 255}
	admColorBorder     = color.NRGBA{R: 222, G: 227, B: 233, A: 255}
	admColorText       = color.NRGBA{R: 32, G: 40, B: 50, A: 255}
	admColorMuted      = color.NRGBA{R: 112, G: 123, B: 137, A: 255}
	admColorPrimary    = color.NRGBA{R: 49, G: 112, B: 211, A: 255}
	admColorDanger     = color.NRGBA{R: 205, G: 66, B: 70, A: 255}
	admColorSuccess    = color.NRGBA{R: 40, G: 146, B: 105, A: 255}
	admColorPreviewBG  = color.NRGBA{R: 18, G: 21, B: 27, A: 255}
	admColorPreviewAlt = admColorPreviewBG
)

func (t admTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return admColorAppBG
	case theme.ColorNameHeaderBackground, theme.ColorNameMenuBackground:
		return admColorPanelBG
	case theme.ColorNameButton:
		return color.NRGBA{R: 238, G: 241, B: 245, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 246, G: 247, B: 249, A: 255}
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return admColorMuted
	case theme.ColorNameError:
		return admColorDanger
	case theme.ColorNameForeground:
		return admColorText
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameFocus:
		return color.NRGBA{R: 49, G: 112, B: 211, A: 70}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return admColorPrimary
	case theme.ColorNameHover, theme.ColorNamePressed:
		return color.NRGBA{R: 224, G: 232, B: 242, A: 180}
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground:
		return admColorPanelBG
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return admColorBorder
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 132, G: 143, B: 155, A: 150}
	case theme.ColorNameScrollBarBackground:
		return color.Transparent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 49, G: 112, B: 211, A: 60}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 25}
	case theme.ColorNameSuccess:
		return admColorSuccess
	case theme.ColorNameWarning:
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255}
	default:
		return t.base.Color(name, theme.VariantLight)
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
		return 8
	case theme.SizeNameInnerPadding:
		return 5
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameText:
		return 13
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return t.base.Size(name)
	}
}

// Secondary copy shares the same type scale without competing with names.
type captionTheme struct{ fyne.Theme }

func (t captionTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameForeground {
		return admColorMuted
	}
	return t.Theme.Color(name, variant)
}
func (t captionTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 11
	}
	return t.Theme.Size(name)
}
