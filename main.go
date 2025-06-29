// main.go
package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"log"
	"os"
	"time"

	"golang.org/x/term"
)

// ---------- configuration ----------
const (
	blockRune         = "█" // character used to draw a pixel
	frameHeightFactor = 2.0 // terminal cells are ~2× taller than wide
)

// ---------- embed fireplace.txt ----------
//go:embed fireplace.txt
var fireplaceB64 string

// ---------- main ----------
func main() {
	frames, delays := decodeGIF()
	clearScreen()

	// --- NEW: Record the start time ---
	startTime := time.Now()

	for {
		for i, img := range frames {
			// --- MODIFIED: Pass start time to render ---
			render(img, startTime)
			time.Sleep(time.Duration(delays[i]) * 10 * time.Millisecond)
		}
	}
}

// ---------- helpers ----------
func decodeGIF() ([]*image.Paletted, []int) {
	raw, err := base64.StdEncoding.DecodeString(fireplaceB64)
	if err != nil {
		log.Fatalf("base64 decode: %v", err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		log.Fatalf("gif decode: %v", err)
	}
	return g.Image, g.Delay
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
		buf.WriteString("\x1b[0m\n")
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
	buf.WriteString(timerText)                                    // Write the text
	buf.WriteString("\x1b[0m")                                    // Reset all styles

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

func clearScreen() { fmt.Print("\x1b[2J") }
