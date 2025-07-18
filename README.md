<p align="center">
  <a href="https://github.com/blacktop/fluxy"><img alt="fluxy Logo" src="https://raw.githubusercontent.com/blacktop/fluxy/main/docs/logo.webp" /></a>
  <h1 align="center">fluxy</h1>
  <h4><p align="center">FLUX image generator TUI</p></h4>
  <p align="center">
    <a href="https://github.com/blacktop/fluxy/actions" alt="Actions">
          <img src="https://github.com/blacktop/fluxy/actions/workflows/go.yml/badge.svg" /></a>
    <a href="https://github.com/blacktop/fluxy/releases/latest" alt="Downloads">
          <img src="https://img.shields.io/github/downloads/blacktop/fluxy/total.svg" /></a>
    <a href="https://github.com/blacktop/fluxy/releases" alt="GitHub Release">
          <img src="https://img.shields.io/github/release/blacktop/fluxy.svg" /></a>
    <a href="http://doge.mit-license.org" alt="LICENSE">
          <img src="https://img.shields.io/:license-mit-blue.svg" /></a>
</p>
<br>

## Why? 🤔

Why leave the terminal to capture an AI image generation idea?

## Getting Started

### Install

```bash
brew install blacktop/tap/fluxy
```

Or

```bash
go install github.com/blacktop/fluxy@latest
```

Or download the latest [release](https://github.com/blacktop/fluxy/releases/latest)

### Run

1) Sign up for an account at [Replicate](https://replicate.com)
2) Place `API_TOKEN` in **env**
      ```bash
      export REPLICATE_API_TOKEN=r8_**********************
      ```
3) exec `fluxy`

```bash
> fluxy --help

FLUX image generator TUI

Usage:
  fluxy [flags]

Flags:
  -t, --api-token string   Replicate API token (overrides REPLICATE_API_KEY env_var)
  -a, --aspect string      Aspect ratio of the image (16:9, 4:3, 1:1, etc) (default "1:1")
  -f, --format string      Output image format (png, webp, or jpg) (default "png")
  -h, --help               help for fluxy
  -m, --model string       Model to use (schnell, pro, or dev) (default "pro")
  -o, --output string      Output folder
  -p, --prompt string      Prompt for image generation
  -V, --verbose            Verbose output
```

![demo](vhs.gif)

> [!WARNING]  
> Currently only the **Kitty** [Terminal Graphics Protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/) works well. Use Ghostty 👻
> You must use a compatible terminal to view these images.

## License

MIT Copyright (c) 2024-2025 **blacktop**