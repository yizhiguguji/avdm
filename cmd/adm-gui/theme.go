package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

type admTheme struct {
	base fyne.Theme
}

// Fixed midnight palette: distinct canvas, raised surfaces and recessed inputs.
var (
	admColorAppBG      = color.NRGBA{R: 11, G: 17, B: 29, A: 255}
	admColorAppBGEnd   = color.NRGBA{R: 17, G: 27, B: 43, A: 255}
	admColorPanelBG    = color.NRGBA{R: 25, G: 36, B: 54, A: 255}
	admColorPanelBG2   = color.NRGBA{R: 33, G: 48, B: 69, A: 255}
	admColorBorder     = color.NRGBA{R: 43, G: 61, B: 83, A: 255}
	admColorText       = color.NRGBA{R: 231, G: 239, B: 249, A: 255}
	admColorMuted      = color.NRGBA{R: 160, G: 180, B: 202, A: 255}
	admColorPrimary    = color.NRGBA{R: 84, G: 99, B: 222, A: 255}
	admColorAccent     = color.NRGBA{R: 64, G: 198, B: 224, A: 255}
	admColorDanger     = color.NRGBA{R: 208, G: 68, B: 84, A: 255}
	admColorSuccess    = color.NRGBA{R: 64, G: 210, B: 166, A: 255}
	admColorPreviewBG  = color.NRGBA{R: 6, G: 11, B: 19, A: 255}
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
		return admColorPanelBG2
	case theme.ColorNameDisabledButton:
		return admColorPanelBG2
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 111, G: 132, B: 157, A: 255}
	case theme.ColorNamePlaceHolder:
		return admColorMuted
	case theme.ColorNameError:
		return admColorDanger
	case theme.ColorNameForeground:
		return admColorText
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameFocus:
		return color.NRGBA{R: 64, G: 198, B: 224, A: 85}
	case theme.ColorNamePrimary:
		return admColorPrimary
	case theme.ColorNameHyperlink:
		return admColorAccent
	case theme.ColorNameHover:
		// Fyne composites this over each button color; an opaque value
		// replaces primary/danger colors and destroys white-text contrast.
		return color.NRGBA{R: 137, G: 190, B: 235, A: 25}
	case theme.ColorNamePressed:
		return color.NRGBA{A: 45}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 16, G: 25, B: 40, A: 255}
	case theme.ColorNameOverlayBackground:
		return admColorPanelBG
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 113, G: 139, B: 169, A: 255}
	case theme.ColorNameSeparator:
		return admColorBorder
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 113, G: 151, B: 185, A: 170}
	case theme.ColorNameScrollBarBackground:
		return color.Transparent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 64, G: 198, B: 224, A: 65}
	case theme.ColorNameShadow:
		return color.NRGBA{A: 100}
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
		return 8
	case theme.SizeNameInnerPadding:
		return 5
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameText:
		return 14
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
		return color.NRGBA{R: 16, G: 27, B: 43, A: 255}
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
