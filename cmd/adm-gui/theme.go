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
	admColorAppBG      = color.NRGBA{R: 220, G: 226, B: 234, A: 255}
	admColorPanelBG    = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	admColorPanelBG2   = color.NRGBA{R: 232, G: 237, B: 244, A: 255}
	admColorBorder     = color.NRGBA{R: 151, G: 164, B: 183, A: 255}
	admColorText       = color.NRGBA{R: 37, G: 42, B: 52, A: 255}
	admColorMuted      = color.NRGBA{R: 65, G: 79, B: 99, A: 255}
	admColorPrimary    = color.NRGBA{R: 82, G: 101, B: 216, A: 255}
	admColorDanger     = color.NRGBA{R: 205, G: 66, B: 70, A: 255}
	admColorSuccess    = color.NRGBA{R: 40, G: 146, B: 105, A: 255}
	admColorPreviewBG  = color.NRGBA{R: 18, G: 21, B: 27, A: 255}
	admColorPreviewAlt = admColorPreviewBG
)

func (t admTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return admColorAppBG
	case theme.ColorNameHeaderBackground:
		return admColorPanelBG2
	case theme.ColorNameMenuBackground:
		return admColorPanelBG
	case theme.ColorNameButton:
		return color.NRGBA{R: 218, G: 226, B: 237, A: 255}
	case theme.ColorNameDisabledButton:
		return admColorPanelBG2
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 124, G: 132, B: 144, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 72, G: 87, B: 108, A: 255}
	case theme.ColorNameError:
		return admColorDanger
	case theme.ColorNameForeground:
		return admColorText
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameFocus:
		return color.NRGBA{R: 82, G: 101, B: 216, A: 70}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return admColorPrimary
	case theme.ColorNameHover:
		// Fyne composites this over each button color; an opaque value
		// replaces primary/danger colors and destroys white-text contrast.
		return color.NRGBA{A: 10}
	case theme.ColorNamePressed:
		return color.NRGBA{A: 20}
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground:
		return admColorPanelBG
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return admColorBorder
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 132, G: 143, B: 155, A: 150}
	case theme.ColorNameScrollBarBackground:
		return color.Transparent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 82, G: 101, B: 216, A: 60}
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
		return 12
	}
	return t.Theme.Size(name)
}

// The toolbar search shares its shape and height with adjacent actions.
type toolbarSearchTheme struct{ fyne.Theme }

func (t toolbarSearchTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 224, G: 231, B: 240, A: 255}
	case theme.ColorNameInputBorder:
		return color.Transparent
	}
	return t.Theme.Color(name, variant)
}
func (t toolbarSearchTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInputRadius {
		return 6
	}
	return t.Theme.Size(name)
}
