package cmd

import "time"

type Input struct {
	Seed     int    `json:"seed,omitempty"`     // Random seed. Set for reproducible generation
	Steps    int    `json:"steps,omitempty"`    // NNumber of diffusion steps
	Prompt   string `json:"prompt,omitempty"`   // Prompt for generated image
	Guidance int    `json:"guidance,omitempty"` // Controls the balance between adherence to the text prompt and image
	// quality/diversity. Higher values make the output more closely match the prompt but may reduce overall image quality.
	//  Lower values allow for more creative freedom but might produce results less relevant to the prompt.
	Interval int `json:"interval,omitempty"` // Interval is a setting that increases the variance in possible outputs
	// letting the model be a tad more dynamic in what outputs it may produce in terms of composition, color, detail, and prompt
	// interpretation. Setting this value low will ensure strong prompt following with more consistent outputs, setting it higher
	// will produce more dynamic or varied outputs.
	NumOutputs    int    `json:"num_outputs,omitempty"`    // Number of outputs to generate
	AspectRatio   string `json:"aspect_ratio,omitempty"`   // Aspect ratio for the generated image
	OutputFormat  string `json:"output_format,omitempty"`  // Format of the output images
	OutputQuality int    `json:"output_quality,omitempty"` // Quality when saving the output images,
	// from 0 to 100. 100 is best quality, 0 is lowest quality. Not relevant for .png outputs
	PromptStrength float32 `json:"prompt_strength,omitempty"` // Prompt strength when using img2img.
	// 1.0 corresponds to full destruction of information in image
	NumInferenceSteps    int  `json:"num_inference_steps,omitempty"`    // Number of denoising steps. Recommended range is 28-50
	DisableSafetyChecker bool `json:"disable_safety_checker,omitempty"` // Disable safety checker for generated images.
	SafetyTolerance      int  `json:"safety_tolerance,omitempty"`       // Safety tolerance, 1 is most strict and 5 is most permissive
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
	Output      any         `json:"output"`
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
