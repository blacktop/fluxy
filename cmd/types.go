package cmd

import "time"

type Input struct {
	Seed          int    `json:"seed,omitempty"`           // Random seed. Set for reproducible generation
	Prompt        string `json:"prompt,omitempty"`         // Prompt for generated image
	NumOutputs    int    `json:"num_outputs,omitempty"`    // Number of outputs to generate
	AspectRatio   string `json:"aspect_ratio,omitempty"`   // Aspect ratio for the generated image
	OutputFormat  string `json:"output_format,omitempty"`  // Format of the output images
	OutputQuality int    `json:"output_quality,omitempty"` // Quality when saving the output images,
	// from 0 to 100. 100 is best quality, 0 is lowest quality. Not relevant for .png outputs
	DisableSafetyChecker bool `json:"disable_safety_checker,omitempty"` // Disable safety checker for generated images.
}

type Response struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Version string `json:"version"`
	Input   struct {
		AspectRatio          string `json:"aspect_ratio"`
		DisableSafetyChecker bool   `json:"disable_safety_checker"`
		NumOutputs           int    `json:"num_outputs"`
		OutputFormat         string `json:"output_format"`
		OutputQuality        int    `json:"output_quality"`
		Prompt               string `json:"prompt"`
		Seed                 int    `json:"seed"`
	} `json:"input"`
	Logs        string      `json:"logs"`
	Output      []string    `json:"output"`
	DataRemoved bool        `json:"data_removed"`
	Error       interface{} `json:"error"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	Urls        struct {
		Cancel string `json:"cancel"`
		Get    string `json:"get"`
	} `json:"urls"`
	Metrics struct {
		ImageCount  int     `json:"image_count"`
		PredictTime float64 `json:"predict_time"`
	} `json:"metrics"`
}
