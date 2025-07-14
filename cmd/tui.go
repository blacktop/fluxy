package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
)

const (
	fluxSchnellURL = "https://api.replicate.com/v1/models/black-forest-labs/flux-schnell/predictions"
	fluxProURL     = "https://api.replicate.com/v1/models/black-forest-labs/flux-1.1-pro-ultra/predictions"
	fluxDevURL     = "https://api.replicate.com/v1/models/black-forest-labs/flux-dev/predictions"
)

// config holds the configuration for the image generation
type config struct {
	Prompt       string
	ApiToken     string
	FluxModel    string
	AspectRatio  string
	OutputFormat string
	OutputFolder string
}

// Color palette
var (
	primaryColor = lipgloss.Color("#7C3AED")
	accentColor  = lipgloss.Color("#06B6D4")
	successColor = lipgloss.Color("#10B981")
	warningColor = lipgloss.Color("#F59E0B")
	errorColor   = lipgloss.Color("#EF4444")
	textColor    = lipgloss.Color("#F8FAFC")
	mutedColor   = lipgloss.Color("#64748B")
	borderColor  = lipgloss.Color("#475569")
)

type newModel struct {
	width         int
	height        int
	prompt        string
	imageData     []byte
	generating    bool
	inputMode     bool
	selectedBtn   int // 0: regenerate, 1: download
	textInput     textinput.Model
	spinner       spinner.Model
	config        *config
	err           error
	imageRendered bool   // Track if image has been rendered
	needsImageClear bool // Flag to force image clearing on next render
	isRegenerating  bool // Track if we're regenerating vs first load
}

func newInitialModel(c *config) newModel {
	ti := textinput.New()
	ti.Placeholder = "Describe the image you want to generate..."
	ti.Focus()
	ti.CharLimit = 200

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	return newModel{
		inputMode:   c.Prompt == "",
		prompt:      c.Prompt,
		textInput:   ti,
		spinner:     s,
		generating:  c.Prompt != "",
		selectedBtn: 0,
		config:      c,
	}
}

func (m newModel) Init() tea.Cmd {
	if m.generating {
		return tea.Batch(generateImage(m.prompt, m.config), m.spinner.Tick)
	}
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m newModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Mark image for re-rendering due to size change
		m.imageRendered = false
		m.needsImageClear = true // Force clear on resize to reposition properly

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.inputMode {
				m.prompt = m.textInput.Value()
				if m.prompt == "" {
					return m, nil
				}
				m.inputMode = false
				m.textInput.Blur() // Remove focus from text input
				m.generating = true
				return m, tea.Batch(generateImage(m.prompt, m.config), m.spinner.Tick)
			} else if m.imageData != nil {
				if m.selectedBtn == 0 {
					// Regenerate: Clear everything and mark for clearing on next render
					termimg.ClearAll()        // Clear all images from terminal immediately
					m.imageData = []byte{}    // Clear cached image data FIRST
					m.needsImageClear = true  // Force clearing on next render
					m.isRegenerating = true   // Mark as regeneration
					m.generating = true
					return m, tea.Batch(tea.ClearScreen, generateImage(m.prompt, m.config), m.spinner.Tick)
				} else {
					// Download
					_, err := saveImage(m.imageData, m.prompt, m.config)
					if err != nil {
						m.err = err
					}
					return m, tea.Quit
				}
			}
		case "left", "h":
			if !m.inputMode && m.imageData != nil {
				m.selectedBtn = 0 // Regenerate
			}
		case "right", "l":
			if !m.inputMode && m.imageData != nil {
				m.selectedBtn = 1 // Download
			}
		case "tab":
			if !m.inputMode && m.imageData != nil {
				m.selectedBtn = (m.selectedBtn + 1) % 2
			}
		case "j", "down":
			// Also handle down/j for consistency
			if !m.inputMode && m.imageData != nil {
				m.selectedBtn = (m.selectedBtn + 1) % 2
			}
		case "k", "up":
			// Also handle up/k for consistency
			if !m.inputMode && m.imageData != nil {
				m.selectedBtn = (m.selectedBtn + 1) % 2
			}
		}

	case tea.MouseClickMsg:
		if !m.inputMode && m.imageData != nil && msg.Button == tea.MouseLeft {
			// Controls panel height is 8, so buttons are in the bottom area
			controlsPanelTop := m.height - 8

			// Check if click is in the controls panel area
			if msg.Y >= controlsPanelTop {
				// Button container is centered and approximately 40 chars wide
				centerX := m.width / 2
				containerWidth := 40
				containerLeft := centerX - containerWidth/2
				containerRight := centerX + containerWidth/2

				if msg.X >= containerLeft && msg.X <= containerRight {
					// Check which button was clicked based on X position
					// Regenerate button is on the left half, Download on the right
					if msg.X < centerX {
						m.selectedBtn = 0
						// Regenerate: Clear everything and mark for clearing on next render
						termimg.ClearAll()        // Clear all images from terminal immediately
						m.imageData = []byte{}    // Clear cached image data FIRST
						m.needsImageClear = true  // Force clearing on next render
						m.isRegenerating = true   // Mark as regeneration
						m.generating = true
						return m, tea.Batch(tea.ClearScreen, generateImage(m.prompt, m.config), m.spinner.Tick)
					} else {
						m.selectedBtn = 1
						// Download
						_, err := saveImage(m.imageData, m.prompt, m.config)
						if err != nil {
							m.err = err
						}
						return m, tea.Quit
					}
				}
			}
		}

	case []byte:
		m.imageData = msg
		m.generating = false
		m.needsImageClear = true // ALWAYS clear on new image data - this fixes regeneration

		// Ensure controls are properly focused when we get image data
		m.selectedBtn = 0  // Default to regenerate button
		m.textInput.Blur() // Ensure text input doesn't have focus

		// Debug logging for troubleshooting regeneration
		debugMsg := fmt.Sprintf("Received NEW image data: %d bytes at %s\n", len(msg), time.Now().Format("15:04:05"))
		os.WriteFile("/tmp/fluxy_update_debug.txt", []byte(debugMsg), 0644)

		return m, nil

	case error:
		m.err = msg
		m.generating = false

		// Optional: Keep debug logging for troubleshooting
		// debugMsg := fmt.Sprintf("Received error: %v\n", msg)
		// os.WriteFile("/tmp/fluxy_error_debug.txt", []byte(debugMsg), 0644)

		return m, nil
	}

	if m.inputMode {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	// Always update spinner and return its command when generating
	var spinnerCmd tea.Cmd
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	if m.generating {
		return m, tea.Batch(cmd, spinnerCmd)
	}

	return m, cmd
}

func (m newModel) View() string {
	// Optional: Debug logging for troubleshooting
	// state := fmt.Sprintf("View called - width:%d, err:%v, inputMode:%v, generating:%v, imageData:%d bytes\n",
	//	m.width, m.err != nil, m.inputMode, m.generating, len(m.imageData))
	// os.WriteFile("/tmp/fluxy_view_debug.txt", []byte(state), 0644)

	if m.width == 0 {
		return "Initializing..."
	}

	if m.err != nil {
		return m.errorView()
	}

	if m.inputMode {
		return m.inputView()
	}

	if m.generating {
		return m.loadingView()
	}

	// Show image view with controls when we have image data
	if m.imageData != nil {
		return m.viewImageWithControls()
	}

	// If no image data and not generating, show controls only
	return m.controlsOnlyView()
}

func (m newModel) inputView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Align(lipgloss.Center).
		Render("✨ FLUXY - AI Image Generator")

	subtitle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Render("Powered by FLUX AI Models")

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1).
		Width(64).
		Align(lipgloss.Center).
		Render(m.textInput.View())

	hint := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Render("Press Enter to generate • Ctrl+C to quit")

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		title,
		"",
		subtitle,
		"",
		"",
		inputBox,
		"",
		hint,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m newModel) loadingView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Align(lipgloss.Center).
		Render("✨ FLUXY")

	// Show different message for regeneration vs first generation
	message := "Generating your image..."
	if m.imageData == nil && m.prompt != "" {
		message = "Regenerating image..."
	}

	spinner := lipgloss.NewStyle().
		Foreground(accentColor).
		Align(lipgloss.Center).
		Render(m.spinner.View() + " " + message)

	subtitle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Render("This may take a few moments")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		spinner,
		"",
		subtitle,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(2).
		Width(50).
		Align(lipgloss.Center).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m newModel) noImageView() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Align(lipgloss.Center).
		Render("✨ FLUXY - AI Image Generator")

	message := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Render("No image generated yet")

	hint := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Center).
		Render("Generate an image to see it here")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		message,
		"",
		hint,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m newModel) controlsOnlyView() string {
	// Simple controls at bottom using escape sequences (no lipgloss borders)
	var b strings.Builder
	
	controlsY := m.height - 6 // Position near bottom
	b.WriteString(m.renderControlsWithEscapes(controlsY))
	
	return b.String()
}

func (m newModel) imageAndControlsView() string {
	// First, render the controls UI normally at the bottom
	controlsPanel := m.renderControlsPanel()
	controlsHeight := lipgloss.Height(controlsPanel)
	
	// Create layout with image area and controls area
	imageAreaHeight := m.height - controlsHeight - 2 // Leave 2 lines margin
	
	// Render controls at bottom with margins
	bottomArea := lipgloss.NewStyle().
		Width(m.width).
		Padding(1, 2). // Add margins so controls don't touch edges
		AlignVertical(lipgloss.Bottom).
		Render(controlsPanel)
	
	// Create the base UI layout
	baseUI := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(m.width).Height(imageAreaHeight).Render(""), // Image space
		bottomArea, // Controls at bottom
	)
	
	// Now overlay the image in the image area using escape sequences
	imageOverlay := m.renderImageOverlay(imageAreaHeight)
	
	// Return base UI first, then image overlay
	return baseUI + imageOverlay
}

func (m *newModel) renderImageOverlay(availableHeight int) string {
	if m.imageData == nil {
		return ""
	}

	img, err := termimg.From(bytes.NewReader(m.imageData))
	if err != nil {
		return ""
	}

	// Calculate image dimensions with proper margins
	imagePadding := 4 // total padding (2 chars each side)
	maxW := m.width - imagePadding
	maxH := availableHeight - 4 // Leave some margin from controls

	// Get native image dimensions and scale appropriately
	bounds := img.Bounds
	origWpx, origHpx := bounds.Dx(), bounds.Dy()
	features := termimg.QueryTerminalFeatures()
	fw, fh := features.FontWidth, features.FontHeight
	origW := int(math.Ceil(float64(origWpx) / float64(fw)))
	origH := int(math.Ceil(float64(origHpx) / float64(fh)))

	// Scale to fit available space
	targetW, targetH := origW, origH
	if origW > maxW || origH > maxH {
		wRatio := float64(maxW) / float64(origW)
		hRatio := float64(maxH) / float64(origH)
		ratio := math.Min(wRatio, hRatio)
		targetW = int(float64(origW) * ratio)
		targetH = int(float64(origH) * ratio)
	}

	img = img.Width(targetW).Height(targetH)

	// Get the image escape sequence
	imageCmd, err := img.Render()
	if err != nil {
		return ""
	}

	var b strings.Builder

	// Clear terminal images if needed
	if m.needsImageClear {
		termimg.ClearAll()
		m.needsImageClear = false
	}

	// Position image in the center of available area with title
	imageY := 3 // Start a few lines down to leave space for title
	imageX := (m.width - targetW) / 2 + 1

	// Add title bar
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor).
		Background(primaryColor).
		Width(m.width).
		Padding(0, 1).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("✨ %s", m.prompt))

	// Position and render image
	b.WriteString("\033[s") // Save cursor position
	b.WriteString(fmt.Sprintf("\033[1;1H")) // Move to top-left
	b.WriteString(title + "\n") // Render title
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageY, imageX)) // Position for image
	b.WriteString(imageCmd) // Render image
	b.WriteString("\033[u") // Restore cursor position

	return b.String()
}

func (m *newModel) viewImageWithControls() string {
	if m.imageData == nil {
		return ""
	}

	img, err := termimg.From(bytes.NewReader(m.imageData))
	if err != nil {
		return m.renderErrorMessage(fmt.Sprintf("Failed to create image: %v", err))
	}

	// Calculate available space for image (leave space for controls at bottom)
	controlsHeight := 8 // Fixed height for controls area
	titleHeight := 1
	availableHeight := m.height - controlsHeight - titleHeight - 2 // Extra margin

	// Calculate image dimensions with margins
	imagePadding := 4
	maxW := m.width - imagePadding
	maxH := availableHeight

	// Scale image appropriately
	bounds := img.Bounds
	origWpx, origHpx := bounds.Dx(), bounds.Dy()
	features := termimg.QueryTerminalFeatures()
	fw, fh := features.FontWidth, features.FontHeight
	origW := int(math.Ceil(float64(origWpx) / float64(fw)))
	origH := int(math.Ceil(float64(origHpx) / float64(fh)))

	targetW, targetH := origW, origH
	if origW > maxW || origH > maxH {
		wRatio := float64(maxW) / float64(origW)
		hRatio := float64(maxH) / float64(origH)
		ratio := math.Min(wRatio, hRatio)
		targetW = int(float64(origW) * ratio)
		targetH = int(float64(origH) * ratio)
	}

	img = img.Width(targetW).Height(targetH)

	// Get image escape sequence
	imageCmd, err := img.Render()
	if err != nil {
		return m.renderErrorMessage(fmt.Sprintf("Failed to render image: %v", err))
	}

	var b strings.Builder

	// Clear terminal images if needed
	if m.needsImageClear {
		termimg.ClearAll()
		m.needsImageClear = false
	}

	// Title bar with escape sequences
	imageY := titleHeight + 3
	imageX := (m.width - targetW) / 2 + 1

	// Render title bar using escape sequences (lipgloss breaks image rendering!)
	b.WriteString(fmt.Sprintf("\033[1;1H")) // Move to top-left
	b.WriteString(fmt.Sprintf("\033[48;2;124;58;237;97m")) // RGB purple background, bright white text
	titleText := fmt.Sprintf("✨ %s", m.prompt)
	padding := (m.width - len(titleText)) / 2
	if padding < 0 { padding = 0 }
	b.WriteString(strings.Repeat(" ", padding))
	b.WriteString(titleText)
	b.WriteString(strings.Repeat(" ", m.width - padding - len(titleText)))
	b.WriteString("\033[0m\n") // Reset colors

	// Position and render image
	b.WriteString("\033[s") // Save cursor
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageY, imageX))
	b.WriteString(imageCmd)
	b.WriteString("\033[u") // Restore cursor

	// Render controls at bottom using escape sequences
	controlsY := m.height - controlsHeight + 2
	b.WriteString(m.renderControlsWithEscapes(controlsY))

	return b.String()
}

func (m newModel) imageView() string {
	if m.imageData == nil {
		return "No image data"
	}

	// Render controls first to get their actual height
	controlsPanel := m.renderControlsPanel()
	controlsHeight := lipgloss.Height(controlsPanel)

	imageHeight := m.height - controlsHeight

	// Create the image panel
	imagePanel := m.renderImagePanel(imageHeight)

	// Join the two panels vertically
	return lipgloss.JoinVertical(lipgloss.Left, imagePanel, controlsPanel)
}

func (m newModel) renderImagePanel(height int) string {
	// Title bar
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor).
		Background(primaryColor).
		Width(m.width).
		Padding(0, 1).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("✨ %s", m.prompt))

	titleHeight := lipgloss.Height(title)
	imageContainerHeight := height - titleHeight

	imageContainer := lipgloss.NewStyle().
		Width(m.width).
		Height(imageContainerHeight).
		Align(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render("") // Leave this empty, the image will be drawn over it

	return lipgloss.JoinVertical(lipgloss.Left, title, imageContainer)
}

func (m newModel) renderControlsPanel() string {
	// Buttons
	regenBtn := "🔄 Regenerate"
	downloadBtn := "💾 Download"

	// Apply selection styling
	if m.selectedBtn == 0 {
		regenBtn = lipgloss.NewStyle().
			Background(warningColor).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Render(regenBtn)
		downloadBtn = lipgloss.NewStyle().
			Foreground(successColor).
			Padding(0, 1).
			Render(downloadBtn)
	} else {
		regenBtn = lipgloss.NewStyle().
			Foreground(warningColor).
			Padding(0, 1).
			Render(regenBtn)
		downloadBtn = lipgloss.NewStyle().
			Background(successColor).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Render(downloadBtn)
	}

	// Create button container with border
	buttonContainer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 2).
		Render(fmt.Sprintf("%s    %s", regenBtn, downloadBtn))

	// Center the button container
	centeredButtons := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(buttonContainer)

	// Hint text
	hint := lipgloss.NewStyle().
		Foreground(mutedColor).
		Width(m.width).
		Align(lipgloss.Center).
		Render("←→/Tab: Navigate • Enter: Execute • Click buttons • Q: Quit")

	// Create controls container with proper height and centering
	controlsContent := lipgloss.JoinVertical(lipgloss.Center,
		centeredButtons,
		"",
		hint,
	)

	// Style the controls panel with a subtle top border
	return lipgloss.NewStyle().
		Width(m.width).
		Padding(1, 0).
		BorderStyle(lipgloss.Border{Top: "─"}).
		BorderForeground(borderColor).
		AlignVertical(lipgloss.Center).
		Render(controlsContent)
}

func (m *newModel) renderFullImageView() string {
	if m.imageData == nil {
		return "" // No image data, nothing to render
	}

	img, err := termimg.From(bytes.NewReader(m.imageData))
	if err != nil {
		// Only show error message if we have significant image data but it failed to parse
		if len(m.imageData) > 100 {
			return m.renderErrorMessage(fmt.Sprintf("Failed to create image (%d bytes): %v", len(m.imageData), err))
		}
		return "" // Small/corrupted data, just skip rendering
	}

	// Protocol detection is handled by go-termimg internally, no need to check here

	// Dynamically calculate available height for the image.
	controlsHeight := lipgloss.Height(m.renderControlsPanel())
	titleHeight := 1 // The title bar is one line high.
	imageHeight := m.height - controlsHeight - titleHeight

	// Ensure calculated height is not negative.
	if imageHeight < 1 {
		return "" // Not enough space to render, just skip
	}

	// Configure the image with the correct dimensions, adding padding
	imagePadding := 4 // 2 chars padding on each side
	imageWidth := max(m.width-imagePadding, 10)

	img = img.Width(imageWidth).Height(imageHeight)

	// Get the raw escape sequence for the image.
	imageCmd, err := img.Render()
	if err != nil {
		return m.renderErrorMessage(fmt.Sprintf("Failed to render image: %v", err))
	}
	if imageCmd == "" {
		return "" // Empty output, just skip
	}

	var b strings.Builder

	// Calculate the correct position for the image
	// The image should be positioned where the imageContainer is rendered
	// Based on the imageView layout:
	// - Title bar takes some lines at the top
	// - Image container starts right after the title
	// - Controls panel is at the bottom

	// Calculate position relative to the COMPLETE UI layout
	ui := m.imageView()
	_ = strings.Count(ui, "\n") // For future use

	// Image should appear OVER the empty imageContainer area within the UI
	// The imageContainer is positioned after the title bar in the imagePanel
	imageY := 2                      // Start after title bar within the UI layout
	imageX := (imagePadding / 2) + 1 // Center with padding

	// Add a subtle title/status line above the image with timestamp to verify regeneration
	timestamp := time.Now().Format("15:04:05")
	b.WriteString(fmt.Sprintf("✨ %s | %dx%d | %d KB | %s\n", m.prompt, imageWidth, imageHeight, len(m.imageData)/1024, timestamp))

	// IMPORTANT: This sequence is critical for correct rendering in a TUI.
	// 1. Clear any previously rendered images.
	termimg.ClearAll()
	// 2. Save the current cursor position.
	b.WriteString("\033[s")
	// 3. Move the cursor to the correct position for the image.
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageY, imageX))
	// 4. Write the image rendering commands.
	b.WriteString(imageCmd)
	// 5. Restore the cursor to its original position. This prevents the image
	//    from disrupting the layout of the UI elements that follow.
	b.WriteString("\033[u")

	// Add controls below the image using escape sequences
	controlsY := imageY + imageHeight + 2 // Position below image

	// Only render controls, not the image itself, to avoid flickering
	b.WriteString(m.renderControlsWithEscapes(controlsY))

	return b.String()
}

func (m *newModel) viewImageOptimized() string {
	if m.imageData == nil {
		return "" // No image data, nothing to render
	}

	img, err := termimg.From(bytes.NewReader(m.imageData))
	if err != nil {
		// Only show error message if we have significant image data but it failed to parse
		if len(m.imageData) > 100 {
			return m.renderErrorMessage(fmt.Sprintf("Failed to create image (%d bytes): %v", len(m.imageData), err))
		}
		return "" // Small/corrupted data, just skip rendering
	}

	// Protocol detection is handled by go-termimg internally, no need to check here

	// Dynamically calculate available height for the image.
	controlsHeight := lipgloss.Height(m.renderControlsPanel())
	titleHeight := 1 // The title bar is one line high.
	imageHeight := m.height - controlsHeight - titleHeight

	// Ensure calculated height is not negative.
	if imageHeight < 1 {
		return "" // Not enough space to render, just skip
	}

	// Determine native vs available dimensions, and resize only if too large
	imagePadding := 4 // total padding (2 chars each side)
	maxW := m.width - imagePadding
	maxH := imageHeight

	// Get native image pixel bounds
	bounds := img.Bounds
	origWpx, origHpx := bounds.Dx(), bounds.Dy()
	// Convert to terminal cell dimensions via font size
	features := termimg.QueryTerminalFeatures()
	fw, fh := features.FontWidth, features.FontHeight
	origW := int(math.Ceil(float64(origWpx) / float64(fw)))
	origH := int(math.Ceil(float64(origHpx) / float64(fh)))

	// Compute target dimensions (preserve aspect ratio)
	// origW/origH now in cell units
	targetW, targetH := origW, origH
	if origW > maxW || origH > maxH {
		wRatio := float64(maxW) / float64(origW)
		hRatio := float64(maxH) / float64(origH)
		ratio := math.Min(wRatio, hRatio)
		targetW = int(float64(origW) * ratio)
		targetH = int(float64(origH) * ratio)
	}

	img = img.Width(targetW).Height(targetH)

	// Get the raw escape sequence for the image.
	imageCmd, err := img.Render()
	if err != nil {
		return m.renderErrorMessage(fmt.Sprintf("Failed to render image: %v", err))
	}
	if imageCmd == "" {
		return "" // Empty output, just skip
	}

	var b strings.Builder

	// Image positioning: vertical offset after title bar with padding
	imageY := titleHeight + 3  // Add extra spacing after title bar
	// Center image horizontally
	imageX := (m.width - targetW) / 2 + 1

	// Add styled title bar with purple background
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor).
		Background(primaryColor).
		Width(m.width).
		Padding(0, 1).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("✨ %s", m.prompt))
	
	b.WriteString(title + "\n")

	// Clear terminal images if needed (for new images or regeneration)
	if m.needsImageClear {
		// Force complete screen clear and cursor reset
		b.WriteString("\033[2J\033[H") // Clear entire screen and move cursor to home
		termimg.ClearAll()             // Clear any terminal image protocols
		m.needsImageClear = false      // Reset the flag after clearing
	}

	// Position and render image
	b.WriteString("\033[s")                                   // Save cursor position
	b.WriteString(fmt.Sprintf("\033[%d;%dH", imageY, imageX)) // Position cursor
	b.WriteString(imageCmd)                                   // Render image
	b.WriteString("\033[u")                                   // Restore cursor position

	// Always add controls below the image - calculate position properly
	controlsY := imageY + targetH + 2 // Use actual image height, not available height
	b.WriteString(m.renderControlsWithEscapes(controlsY))

	return b.String()
}

func (m *newModel) updateControlsOnly() string {
	// Just update the controls without re-rendering the image
	// Calculate where controls should be positioned
	if m.imageData == nil {
		return ""
	}

	// Calculate image dimensions (same logic as renderFullImageView)
	controlsHeight := lipgloss.Height(m.renderControlsPanel())
	titleHeight := 1
	imageHeight := m.height - controlsHeight - titleHeight

	if imageHeight < 1 {
		return ""
	}

	imageY := 2
	controlsY := imageY + imageHeight + 2

	// ONLY update the controls area - don't touch the image or title
	return m.renderControlsWithEscapes(controlsY)
}

func (m *newModel) renderControlsWithEscapes(controlsY int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\033[%d;1H", controlsY)) // Move to controls position

	// Create simple controls with selection highlighting
	regenBtn := "🔄 Regenerate"
	downloadBtn := "💾 Download"

	if m.selectedBtn == 0 {
		regenBtn = fmt.Sprintf("\033[43;30m %s \033[0m", regenBtn)    // Yellow background for selected
		downloadBtn = fmt.Sprintf("\033[37m %s \033[0m", downloadBtn) // Gray for unselected
	} else {
		regenBtn = fmt.Sprintf("\033[37m %s \033[0m", regenBtn)          // Gray for unselected
		downloadBtn = fmt.Sprintf("\033[42;30m %s \033[0m", downloadBtn) // Green background for selected
	}

	controls := fmt.Sprintf("  %s    %s    Press Enter to execute • ←→ to navigate • Q to quit", regenBtn, downloadBtn)
	b.WriteString(controls)

	return b.String()
}

func (m *newModel) renderErrorMessage(message string) string {
	// Add terminal info to help with debugging
	terminalInfo := fmt.Sprintf("Terminal: %s", os.Getenv("TERM"))
	if os.Getenv("TERM_PROGRAM") != "" {
		terminalInfo += fmt.Sprintf(" (%s)", os.Getenv("TERM_PROGRAM"))
	}

	// Add protocol info
	protocol := termimg.DetectProtocol()
	protocolInfo := fmt.Sprintf("Protocol: %s", protocol.String())

	// Create a styled error message box
	errorContent := fmt.Sprintf("🚨 Image Error\n\n%s\n\n%s\n%s", message, terminalInfo, protocolInfo)

	errorBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(errorColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(errorColor).
		Padding(1, 2).
		Width(min(70, m.width-4)).
		Align(lipgloss.Center).
		Render(errorContent)

	// Position the error box in the center of the image area
	if m.height > 0 && m.width > 0 {
		controlsHeight := lipgloss.Height(m.renderControlsPanel())
		titleHeight := 1
		imageHeight := m.height - controlsHeight - titleHeight

		if imageHeight > 0 {
			return lipgloss.Place(m.width, imageHeight, lipgloss.Center, lipgloss.Center, errorBox)
		}
	}

	return errorBox
}

// bottomUI function removed - using inline button rendering

func (m newModel) errorView() string {
	errorBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(errorColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(errorColor).
		Padding(1).
		Width(60).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("Error: %v", m.err))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errorBox)
}

// max function removed - no longer needed

// generateImage generates an image using the Replicate API
func generateImage(prompt string, c *config) tea.Cmd {
	return func() tea.Msg {
		var apiKey string
		if c.ApiToken != "" {
			apiKey = c.ApiToken
		} else {
			apiKey = os.Getenv("REPLICATE_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("replicate API token not provided. Use --api-token flag or set REPLICATE_API_KEY environment variable")
		}

		input := Input{
			Prompt:        prompt,
			AspectRatio:   c.AspectRatio,
			OutputFormat:  c.OutputFormat,
			OutputQuality: 100,
		}

		var fluxURL string
		switch c.FluxModel {
		case "schnell":
			fluxURL = fluxSchnellURL
			input.DisableSafetyChecker = true
		case "pro":
			fluxURL = fluxProURL
			input.SafetyTolerance = 5
		case "dev":
			fluxURL = fluxDevURL
		default:
			return fmt.Errorf("invalid flux model: %s", c.FluxModel)
		}

		jsonPayload, err := json.Marshal(map[string]Input{"input": input})
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}

		req, err := http.NewRequest("POST", fluxURL, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %w", err)
		}

		var result Response
		err = json.Unmarshal(body, &result)
		if err != nil {
			return fmt.Errorf("error unmarshaling JSON: %w", err)
		}

		log.Debug("API response", "body", string(body)+"\n")

		// Poll the API for the final result
		for result.Status != "succeeded" && result.Status != "failed" {
			time.Sleep(1 * time.Second)

			req, err := http.NewRequest("GET", result.Urls.Get, nil)
			if err != nil {
				return fmt.Errorf("error creating request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("error sending request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("error reading response: %w", err)
			}

			log.Debug("API response", "body", string(body)+"\n")

			err = json.Unmarshal(body, &result)
			if err != nil {
				return fmt.Errorf("error unmarshaling JSON: %w", err)
			}

			log.Debug("Polling API", "status", result.Status)
		}

		if result.Status == "failed" {
			return fmt.Errorf("image generation failed: %s", result.Error)
		}

		// Fetch the generated image
		var outputURL string
		if url, ok := result.Output.(string); ok {
			outputURL = url
		} else if urls, ok := result.Output.([]any); ok {
			outputURL = urls[0].(string)
		} else {
			return fmt.Errorf("unexpected output type: %T", result.Output)
		}

		resp, err = http.Get(outputURL)
		if err != nil {
			return fmt.Errorf("error fetching image: %w", err)
		}
		defer resp.Body.Close()

		imageData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading image data: %w", err)
		}

		return imageData // Return the image data directly
	}
}

// saveImage saves the generated image to disk
func saveImage(imageData []byte, prompt string, config *config) (string, error) {
	// Sanitize the prompt for use in a filename
	sanitizedPrompt := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, prompt)

	// Truncate the sanitized prompt if it's too long
	if len(sanitizedPrompt) > 50 {
		sanitizedPrompt = sanitizedPrompt[:50]
	}

	filename := fmt.Sprintf("%s_%d.%s", sanitizedPrompt, time.Now().Unix(), config.OutputFormat)
	if config.OutputFolder != "" {
		if err := os.MkdirAll(config.OutputFolder, 0755); err != nil {
			return "", fmt.Errorf("error creating output folder: %w", err)
		}
		filename = filepath.Join(config.OutputFolder, filename)
	}

	// Use the original image data for saving
	if err := os.WriteFile(filename, imageData, 0644); err != nil {
		return "", fmt.Errorf("error saving image: %w", err)
	}
	fmt.Printf("✨ Image saved: %s\n", filename)
	return filename, nil
}
