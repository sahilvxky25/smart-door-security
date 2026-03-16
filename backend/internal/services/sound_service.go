package services

import (
	"log"
	"syscall"
	"time"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	procBeep = kernel32.NewProc("Beep")
)

type SoundService struct{}

func NewSoundService() *SoundService {
	return &SoundService{}
}

// playBeep calls the Windows Beep API synchronously (blocks for the duration).
func playBeep(freqHz, durationMs uint) {
	procBeep.Call(uintptr(freqHz), uintptr(durationMs))
}

// PlayWelcome plays a cheerful ascending 3-note chime when an authorized user is recognized.
// Runs in a goroutine so it never blocks the caller.
func (s *SoundService) PlayWelcome() {
	log.Println("[SoundService] Playing welcome chime")
	go func() {
		// C5 → E5 → G5 ascending chime
		playBeep(523, 150) // C5
		time.Sleep(60 * time.Millisecond)
		playBeep(659, 150) // E5
		time.Sleep(60 * time.Millisecond)
		playBeep(784, 350) // G5
	}()
}

// PlaySOS plays an SOS (· · · — — — · · ·) Morse pattern on the system speaker.
// Runs in a goroutine so it never blocks the caller.
func (s *SoundService) PlaySOS() {
	log.Println("[SoundService] Playing SOS alert")
	go func() {
		const (
			freq    = 900 // Hz
			dot     = 150 // ms
			dash    = 450 // ms
			elemGap = 150 // ms between elements within a letter
			charGap = 450 // ms between letters
		)

		// S: · · ·
		playBeep(freq, dot)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dot)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dot)
		time.Sleep(charGap * time.Millisecond)

		// O: — — —
		playBeep(freq, dash)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dash)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dash)
		time.Sleep(charGap * time.Millisecond)

		// S: · · ·
		playBeep(freq, dot)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dot)
		time.Sleep(elemGap * time.Millisecond)
		playBeep(freq, dot)
	}()
}
