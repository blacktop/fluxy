/*
Copyright © 2024 blacktop

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var (
	// flags
	logger          *log.Logger
	verbose         bool
	displayProtocol string
	aspectRatio     string
	outputFormat    string
	outputFolder    string
	// choices
	validDisplayProtocols = []string{
		"kitty",
		"iterm",
	}
	validOutputFormats = []string{
		"png",
		"webp",
		"jpg",
	}
	validAspectRatios = []string{
		"1:1",
		"16:9",
		"21:9",
		"2:3",
		"3:2",
		"4:5",
		"5:4",
		"9:16",
		"9:21",
	}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "fluxy",
	Short: "FLUX image generator TUI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// flags
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
		// validate flags
		if !slices.Contains(validAspectRatios, aspectRatio) {
			logger.Error(fmt.Sprintf("Invalid aspect ratio (must be one of: %s)", strings.Join(validAspectRatios, ", ")), "aspect", aspectRatio)
			os.Exit(1)
		}
		if !slices.Contains(validOutputFormats, outputFormat) {
			logger.Error(fmt.Sprintf("Invalid output format (must be one of: %s)", strings.Join(validOutputFormats, ", ")), "format", outputFormat)
			os.Exit(1)
		}
		if !slices.Contains(validDisplayProtocols, displayProtocol) {
			logger.Error(fmt.Sprintf("Invalid display protocol (must be one of: %s)", strings.Join(validDisplayProtocols, ", ")), "display", displayProtocol)
			os.Exit(1)
		}
		// run
		p := tea.NewProgram(initialModel(&config{
			DisplayProtocol: displayProtocol,
			AspectRatio:     aspectRatio,
			OutputFormat:    outputFormat,
			OutputFolder:    outputFolder,
		}), tea.WithAltScreen())
		m, err := p.Run()
		if err != nil {
			logger.Error("Error running program", "error", err)
			os.Exit(1)
		}
		if m, ok := m.(model); ok {
			if len(m.saved) > 0 {
				println() // add space
				logger.Infof("Saved image to %s", m.saved)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Override the default error level style.
	styles := log.DefaultStyles()
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERROR!!").
		Padding(0, 1, 0, 1).
		Background(lipgloss.Color("204")).
		Foreground(lipgloss.Color("0"))
	// Add a custom style for key `err`
	styles.Keys["err"] = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	styles.Values["err"] = lipgloss.NewStyle().Bold(true)
	logger = log.New(os.Stderr)
	logger.SetStyles(styles)

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Verbose output")
	rootCmd.Flags().StringVarP(&displayProtocol, "display", "d", "kitty", "Terminal graphics protocol to use (kitty or iterm)")
	rootCmd.Flags().StringVarP(&aspectRatio, "aspect", "a", "1:1", "Aspect ratio of the image (example: 16:9, 4:3, 1:1)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "png", "Output image format (png, webp, or jpg)")
	rootCmd.Flags().StringVarP(&outputFolder, "output", "o", "", "Output folder")
	rootCmd.MarkFlagDirname("output")
}
