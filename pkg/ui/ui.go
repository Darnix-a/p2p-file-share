package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// FormatBytes formats byte counts into human-readable strings (KB, MB, GB)
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// NewProgressBar creates a modern progress bar for file transfer
func NewProgressBar(totalBytes int64, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		totalBytes,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(18),
		progressbar.OptionThrottle(65),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

// PromptConfirm asks the user for confirmation (y/N)
func PromptConfirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// PrintBanner displays the tool header banner
func PrintBanner() {
	banner := `
  ____ ____  ____        ____  ____   ___  ____  
 |  _ \___ \|  _ \      |  _ \|  _ \ / _ \|  _ \ 
 | |_) |__) | |_) |_____| | | | |_) | | | | |_) |
 |  __// __/|  __/ _____| |_| |  _ <| |_| |  __/ 
 |_|  |_____|_|         |____/|_| \_\\___/|_|    
    End-to-End Encrypted P2P File Dropper
`
	fmt.Print(banner)
	fmt.Println()
}
