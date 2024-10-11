package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

const (
	fluxSchnellURL = "https://api.replicate.com/v1/models/black-forest-labs/flux-schnell/predictions"
	fluxProURL     = "https://api.replicate.com/v1/models/black-forest-labs/flux-pro/predictions"
	fluxPro1_1URL  = "https://api.replicate.com/v1/models/black-forest-labs/flux-1.1-pro/predictions"
	fluxDevURL     = "https://api.replicate.com/v1/models/black-forest-labs/flux-dev/predictions"
)

type config struct {
	Prompt       string
	ApiToken     string
	FluxModel    string
	AspectRatio  string
	OutputFormat string
	OutputFolder string
}

type model struct {
	prompt       string
	image        *termimg.TermImg
	err          error
	inputMode    bool
	cursorPos    int
	buttonMode   int // 0: none, 1: download, 2: regenerate
	textInput    textinput.Model
	viewport     viewport.Model
	width        int
	height       int
	spinner      spinner.Model
	generating   bool
	config       *config
	saved        string
	regenerating bool
}

func initialModel(c *config) model {
	ti := textinput.New()
	ti.Placeholder = "Enter prompt"
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{
		inputMode:  c.Prompt == "",
		textInput:  ti,
		viewport:   viewport.New(0, 0),
		spinner:    s,
		generating: c.Prompt != "",
		config:     c,
	}

	if c.Prompt != "" {
		m.prompt = c.Prompt
	}

	return m
}

func (m model) Init() tea.Cmd {
	if m.generating {
		return tea.Batch(generateImage(m.prompt, m.config), m.spinner.Tick)
	}
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = int(float64(m.width) * 0.6)
		m.viewport.Height = m.height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.inputMode {
				m.prompt = m.textInput.Value()
				m.inputMode = false
				m.generating = true
				return m, tea.Batch(generateImage(m.prompt, m.config), m.spinner.Tick)
			} else {
				switch m.buttonMode {
				case 1:
					log.Debug("Downloading image")
					m.saved, m.err = saveImage(m.image, m.prompt)
					return m, tea.Quit
				case 2:
					log.Debug("Regenerating image", "prompt", m.prompt)
					m.regenerating = true
					m.generating = true
					m.image = nil             // Clear the existing image
					m.viewport.SetContent("") // Clear the viewport content
					return m, tea.Batch(generateImage(m.prompt, m.config), m.spinner.Tick)
				}
			}
		case "tab":
			if !m.inputMode {
				m.buttonMode = (m.buttonMode + 1) % 3
			}
		case "backspace":
			if m.inputMode && len(m.prompt) > 0 {
				m.prompt = m.prompt[:len(m.prompt)-1]
				m.cursorPos--
			}
		default:
			if m.inputMode {
				m.prompt += msg.String()
				m.cursorPos++
			}
		}
	case []byte:
		img, err := termimg.NewTermImg(bytes.NewReader(msg))
		if err != nil {
			m.err = err
		}
		m.image = img
		m.generating = false
		m.regenerating = false
		return m, nil
	case error:
		m.err = msg
		return m, tea.Quit
	case string:
		log.Debug("Message received", "message", msg)
	}
	m.textInput, cmd = m.textInput.Update(msg)
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if m.width == 0 {
		return "Initializing..."
	}

	leftWidth := int(float64(m.width) * 0.4)
	rightWidth := m.width - leftWidth

	leftPanel := m.leftPanelView(leftWidth)
	rightPanel := m.rightPanelView(rightWidth)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	if m.generating && (m.image == nil || m.regenerating) {
		return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(
			lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.spinnerPopup(), lipgloss.WithWhitespaceChars("  "),
				lipgloss.WithWhitespaceForeground(lipgloss.Color("0"))),
		)
	}

	return mainView
}

func (m model) spinnerPopup() string {
	if !m.generating || (m.image != nil && !m.regenerating) {
		return ""
	}

	spinnerWidth := 40
	spinnerHeight := 3

	style := lipgloss.NewStyle().
		Width(spinnerWidth).
		Height(spinnerHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Align(lipgloss.Center, lipgloss.Center)

	content := fmt.Sprintf("%s Generating image...", m.spinner.View())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, style.Render(content))
}

func (m model) leftPanelView(width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(m.height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true)

	content := ""
	if m.inputMode {
		content = fmt.Sprintf("Enter prompt:\n\n%s", m.textInput.View())
	} else if m.image != nil {
		downloadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
		regenerateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

		if m.buttonMode == 1 {
			downloadStyle = downloadStyle.Background(lipgloss.Color("7"))
		} else if m.buttonMode == 2 {
			regenerateStyle = regenerateStyle.Background(lipgloss.Color("7"))
		}

		content = fmt.Sprintf(
			"Prompt: %s\n\n%s\n%s",
			m.prompt,
			downloadStyle.Render("[ Download ]"),
			regenerateStyle.Render("[ Regenerate ]"),
		)
	} else {
		content = "Generating image..."
	}

	return style.Render(content)
}

func (m model) rightPanelView(width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(m.height)

	if m.image != nil && !m.regenerating {
		cmd, _ := m.image.Render()
		m.viewport.SetContent(cmd)
		centeredContent := lipgloss.Place(width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.viewport.View())
		return style.Render(centeredContent)
	}

	placeholderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Align(lipgloss.Center, lipgloss.Center).
		Width(width).
		Height(m.height)

	if m.regenerating {
		return style.Render("") // Return an empty string to clear the panel
	}

	return placeholderStyle.Render("Image will be displayed here")
}

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

func saveImage(image *termimg.TermImg, prompt string) (string, error) {
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

	filename := fmt.Sprintf("%s_%d.png", sanitizedPrompt, time.Now().Unix())
	if outputFolder != "" {
		if err := os.MkdirAll(outputFolder, 0755); err != nil {
			return "", fmt.Errorf("error creating output folder: %w", err)
		}
		filename = filepath.Join(outputFolder, filename)
	}
	data, err := image.AsPNGBytes()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("error saving image: %w", err)
	}
	fmt.Printf("Image saved: %s\n", filename)
	return filename, nil
}
