package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"flag"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var verbose bool

func init() {
	flag.BoolVar(&verbose, "V", false, "Enable verbose logging")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
}

type model struct {
	prompt     string
	image      []byte
	err        error
	inputMode  bool
	cursorPos  int
	buttonMode int // 0: none, 1: download, 2: regenerate
	textInput  textinput.Model
	viewport   viewport.Model
	width      int
	height     int
	spinner    spinner.Model
	generating bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter prompt"
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		inputMode:  true,
		textInput:  ti,
		viewport:   viewport.New(0, 0),
		spinner:    s,
		generating: false,
	}
}

func (m model) Init() tea.Cmd {
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
				return m, tea.Batch(generateImage(m.prompt), m.spinner.Tick)
			} else {
				switch m.buttonMode {
				case 1:
					log.Debug("Downloading image")
					return m, downloadImage(m.image, m.prompt)
				case 2:
					log.Debug("Regenerating image", "prompt", m.prompt)
					return m, generateImage(m.prompt)
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
		m.image = msg
		m.generating = false
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

	if m.generating {
		return fmt.Sprintf("%s Generating image for prompt: %s", m.spinner.View(), m.prompt)
	}

	if m.width == 0 {
		return "Initializing..."
	}

	leftWidth := int(float64(m.width) * 0.4)
	rightWidth := m.width - leftWidth

	leftPanel := m.leftPanelView(leftWidth)
	rightPanel := m.rightPanelView(rightWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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
	} else if len(m.image) > 0 {
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

	if len(m.image) > 0 {
		cmd := displayITermImage(m.image)
		m.viewport.SetContent(cmd)
		return style.Render(m.viewport.View())
	}

	return style.Render("Image will be displayed here")
}

func displayKittyImage(image []byte) string {
	encoded := base64.StdEncoding.EncodeToString(image)
	return fmt.Sprintf("\033_Ga=T,f=100;%s\033\\", encoded)
}

func displayITermImage(image []byte) string {
	encoded := base64.StdEncoding.EncodeToString(image)
	return fmt.Sprintf("\033]1337;File=inline=1;size=%d;width=auto;height=auto:%s\a\n", len(image), encoded)
}

func generateImage(prompt string) tea.Cmd {
	return func() tea.Msg {
		apiKey := os.Getenv("REPLICATE_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("REPLICATE_API_KEY environment variable not set")
		}

		payload := map[string]Input{
			"input": {
				Prompt:     prompt,
				NumOutputs: 1,
				// AspectRatio:          "1:1",
				AspectRatio:          "16:9",
				OutputFormat:         "webp",
				OutputQuality:        100,
				DisableSafetyChecker: true,
			},
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}

		req, err := http.NewRequest("POST", "https://api.replicate.com/v1/models/black-forest-labs/flux-schnell/predictions", bytes.NewBuffer(jsonPayload))
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
		resp, err = http.Get(result.Output[0])
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

func downloadImage(image []byte, prompt string) tea.Cmd {
	return func() tea.Msg {
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
		err := os.WriteFile(filename, image, 0644)
		if err != nil {
			return fmt.Errorf("error saving image: %w", err)
		}
		log.Debug("Image saved", "filename", filename)
		return fmt.Sprintf("Image saved as %s", filename)
	}
}
