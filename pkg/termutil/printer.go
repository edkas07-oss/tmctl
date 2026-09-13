package termutil

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[1;33m"
	ColorBlue   = "\033[0;34m"
	ColorCyan   = "\033[0;36m"
	ColorBold   = "\033[1m"
)

var noColor = false

func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		noColor = true
	}
}

// SetNoColor disables color output explicitly.
func SetNoColor(disable bool) {
	noColor = disable
}

func colorize(color, msg string) string {
	if noColor {
		return msg
	}
	return color + msg + ColorReset
}

// Success prints a green success message.
func Success(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s %s\n", colorize(ColorGreen, "✔ SUCCESS:"), msg)
}

// Info prints a blue information message.
func Info(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s %s\n", colorize(ColorBlue, "ℹ INFO:"), msg)
}

// Warn prints a yellow warning message.
func Warn(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s %s\n", colorize(ColorYellow, "⚠ WARN:"), msg)
}

// Error prints a red error message to stderr.
func Error(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s %s\n", colorize(ColorRed, "✘ ERROR:"), msg)
}

// Step prints a cyan step indicator.
func Step(step int, total int, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	prefix := fmt.Sprintf("[%d/%d]", step, total)
	fmt.Printf("%s %s\n", colorize(ColorCyan, prefix), msg)
}

// PrintTable formats tabular data neatly.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	headerLine := strings.Join(headers, "\t")
	fmt.Fprintln(tw, colorize(ColorBold, headerLine))
	
	// Separator
	var seps []string
	for _, h := range headers {
		seps = append(seps, strings.Repeat("-", len(h)+2))
	}
	fmt.Fprintln(tw, strings.Join(seps, "\t"))

	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}
