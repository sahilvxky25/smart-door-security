package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	out, _ := cmd.CombinedOutput()

	fmt.Println("--- FFMPEG DEVICE OUTPUT ---")
	fmt.Println(string(out))
	fmt.Println("--- PARSED AUDIO DEVICES ---")

	section := ""
	re := regexp.MustCompile(`\]\s+"(.+)"`)
	altRe := regexp.MustCompile(`(?i)alternative name`)

	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "directshow video devices") {
			section = "video"
			continue
		}
		if strings.Contains(lower, "directshow audio devices") {
			section = "audio"
			continue
		}
		if altRe.MatchString(line) {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			devName := m[1]
			if section == "audio" {
				fmt.Printf("- %s\n", devName)
			}
		}
	}
}
