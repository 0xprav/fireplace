// main.go
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

// ---------- configuration ----------
const (
	blockRune         = "█" // character used to draw a pixel
	frameHeightFactor = 2.0 // terminal cells are ~2× taller than wide
)

// ---------- embed fireplace.txt ----------
//
//go:embed fireplace.txt
var fireplaceB64 string

// ---------- main ----------
func main() {
	frames, delays := decodeGIF()

	ctx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)
	defer stopSignals()

	restoreTerminal := useAlternateScreen()
	defer restoreTerminal()

	stopAwake := keepAwake()
	defer stopAwake()

	startTime := time.Now()

	for {
		for i, img := range frames {
			render(img, startTime)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(delays[i]) * 10 * time.Millisecond):
			}
		}
	}
}

// ---------- helpers ----------
func decodeGIF() ([]image.Image, []int) {
	raw, err := base64.StdEncoding.DecodeString(fireplaceB64)
	if err != nil {
		log.Fatalf("base64 decode: %v", err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		log.Fatalf("gif decode: %v", err)
	}
	return compositeFrames(g), g.Delay
}

func compositeFrames(g *gif.GIF) []image.Image {
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	canvas := image.NewRGBA(bounds)
	frames := make([]image.Image, 0, len(g.Image))

	for _, frame := range g.Image {
		// GIF frames can contain transparent pixels that retain pixels from the
		// prior frame. Draw each frame over the canvas before saving it.
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		composite := image.NewRGBA(bounds)
		draw.Draw(composite, bounds, canvas, bounds.Min, draw.Src)
		frames = append(frames, composite)
	}

	return frames
}

func keepAwake() (stop func()) {
	cmd := exec.Command("caffeinate", "-dimsu")
	_ = cmd.Start()

	return func() {
		_ = cmd.Process.Kill()
	}
}

// --- MODIFIED: render now accepts startTime to calculate and display the timer ---
func render(img image.Image, startTime time.Time) {
	// Get terminal dimensions
	termCols, termRows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termCols == 0 || termRows == 0 {
		termCols, termRows = 80, 24 // Sensible fallback
	}

	// Get image dimensions
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	// Stretch scaling factors
	scaleX := float64(imgW) / float64(termCols)
	scaleY := float64(imgH) / float64(termRows)

	// Use a buffer for flicker-free rendering
	var buf bytes.Buffer
	buf.WriteString("\x1b[H") // Move cursor to top-left

	// Render the stretched image frame
	for y := 0; y < termRows; y++ {
		for x := 0; x < termCols; x++ {
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)
			r, g, b, _ := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY).RGBA()
			buf.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s", r>>8, g>>8, b>>8, blockRune))
		}
		buf.WriteString("\x1b[0m")
		if y < termRows-1 {
			buf.WriteByte('\n')
		}
	}

	// --- NEW: Timer Overlay Logic ---
	// 1. Calculate and format the timer text
	elapsed := time.Since(startTime)
	timerText := formatDuration(elapsed)

	// 2. Calculate position for the text (bottom-center)
	// ANSI cursor positions are 1-based, so row is termRows
	textCol := (termCols - len(timerText)) / 2
	if textCol < 1 {
		textCol = 1 // Ensure it's at least 1
	}

	// 3. Append ANSI codes to the buffer to draw the text
	buf.WriteString(fmt.Sprintf("\x1b[%d;%dH", termRows, textCol)) // Move cursor to position
	buf.WriteString("\x1b[38;2;255;255;255;48;2;0;0;0m")           // Set style: White text on Black background
	buf.WriteString(timerText)                                     // Write the text
	buf.WriteString("\x1b[0m")                                     // Reset all styles

	// Print the entire buffer (image + text overlay) at once
	fmt.Print(buf.String())
}

// --- NEW: Helper function to format duration as HH:MM:SS ---
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("Uptime: %02d:%02d:%02d", h, m, s)
}

func useAlternateScreen() (restore func()) {
	// The alternate screen keeps animation frames out of the terminal history.
	// Leaving it restores the screen that was visible before Fireplace started.
	fmt.Print("\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H")

	return func() {
		fmt.Print("\x1b[0m\x1b[?25h\x1b[?1049l")
	}
}
