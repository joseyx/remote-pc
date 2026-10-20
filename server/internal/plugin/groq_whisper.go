package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	GroqTranscriptionURL = "https://api.groq.com/openai/v1/audio/transcriptions"
	MaxAudioSizeBytes    = 24 * 1024 * 1024 // 24 MB (Groq limit is 25 MB)
	ChunkDurationSeconds = 720              // 12 minutes per chunk if oversized
)

type groqVerboseResponse struct {
	Task     string  `json:"task"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Text     string  `json:"text"`
	Segments []struct {
		ID    int     `json:"id"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// CheckFFmpeg verifies that ffmpeg is installed and accessible
func CheckFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("FFmpeg no encontrado en el PATH del sistema. Instálalo para habilitar la extracción de audio")
	}
	return nil
}

// ExtractAudio converts video/audio to a lightweight mono MP3 optimized for Whisper
func ExtractAudio(ctx context.Context, mediaPath, outputPath, bitrate string) error {
	if err := CheckFFmpeg(); err != nil {
		return err
	}
	if bitrate == "" {
		bitrate = "48k"
	}

	// -vn: no video
	// -acodec libmp3lame: mp3 audio
	// -ar 16000: 16 kHz sample rate (optimal for Whisper)
	// -ac 1: mono
	// -b:a 48k: 48kbps bitrate
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", mediaPath, "-vn", "-acodec", "libmp3lame", "-ar", "16000", "-ac", "1", "-b:a", bitrate, outputPath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al extraer audio con ffmpeg: %w: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

// getMediaDuration returns total duration of an audio/video file in seconds
func getMediaDuration(ctx context.Context, mediaPath string) (float64, error) {
	// Try ffprobe first
	if _, err := exec.LookPath("ffprobe"); err == nil {
		cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", mediaPath)
		out, err := cmd.Output()
		if err == nil {
			str := strings.TrimSpace(string(out))
			if d, parseErr := strconv.ParseFloat(str, 64); parseErr == nil && d > 0 {
				return d, nil
			}
		}
	}

	// Fallback to ffmpeg output parsing
	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", mediaPath)
	out, _ := cmd.CombinedOutput()
	str := string(out)
	idx := strings.Index(str, "Duration: ")
	if idx != -1 {
		part := str[idx+len("Duration: "):]
		commaIdx := strings.Index(part, ",")
		if commaIdx != -1 {
			timeStr := strings.TrimSpace(part[:commaIdx])
			parts := strings.Split(timeStr, ":")
			if len(parts) == 3 {
				h, _ := strconv.ParseFloat(parts[0], 64)
				m, _ := strconv.ParseFloat(parts[1], 64)
				s, _ := strconv.ParseFloat(parts[2], 64)
				return (h * 3600) + (m * 60) + s, nil
			}
		}
	}
	return 0, fmt.Errorf("no se pudo determinar la duración del archivo multimedia")
}

// sendGroqWhisperRequest uploads a single audio file to Groq Whisper API
func sendGroqWhisperRequest(ctx context.Context, apiKey, model, language, prompt, audioPath string) (*groqVerboseResponse, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("error al leer archivo de audio: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	if model == "" {
		model = "whisper-large-v3-turbo"
	}
	_ = writer.WriteField("model", model)
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("temperature", "0.0")

	if language != "" && language != "auto" {
		_ = writer.WriteField("language", language)
	}
	if prompt != "" {
		_ = writer.WriteField("prompt", prompt)
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GroqTranscriptionURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error conectando con Groq API: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de Groq: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var groqErr groqVerboseResponse
		_ = json.Unmarshal(respBytes, &groqErr)
		if groqErr.Error != nil && groqErr.Error.Message != "" {
			return nil, fmt.Errorf("Groq API error (%d): %s", resp.StatusCode, groqErr.Error.Message)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("Groq API Key inválida o no autorizada (HTTP 401)")
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("límite de peticiones alcanzado en Groq API (HTTP 429)")
		}
		return nil, fmt.Errorf("Groq API error HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed groqVerboseResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta JSON de Groq: %w", err)
	}

	return &parsed, nil
}

// formatTimestamp converts seconds into [MM:SS] or [HH:MM:SS]
func formatTimestamp(seconds float64) string {
	totalSec := int(seconds)
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// TranscribeMedia processes an audio or video file, handles extraction and chunking, and returns the transcription
func TranscribeMedia(ctx context.Context, mediaPath string, withTimestamps bool, customPrompt, apiKey, model, language string) (*TranscriptionResult, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("Groq API Key no configurada. Añádela en Orbit > Plugins > Groq Whisper Transcriber")
	}

	mediaPath, err := filepath.Abs(filepath.Clean(mediaPath))
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(mediaPath); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("archivo multimedia no encontrado: %s", mediaPath)
	}

	// Create temporary MP3 for processing
	tempDir := os.TempDir()
	tempAudioPath := filepath.Join(tempDir, fmt.Sprintf("orbit_transcribe_%d.mp3", time.Now().UnixNano()))
	defer os.Remove(tempAudioPath)

	// Step 1: Extract lightweight audio
	if err := ExtractAudio(ctx, mediaPath, tempAudioPath, "48k"); err != nil {
		return nil, err
	}

	audioStat, err := os.Stat(tempAudioPath)
	if err != nil {
		return nil, fmt.Errorf("error al verificar audio extraído: %w", err)
	}

	var combinedResult TranscriptionResult
	var allSegments []Segment
	var fullTextBuilder strings.Builder

	// Step 2: Check size and chunk if needed
	if audioStat.Size() <= MaxAudioSizeBytes {
		// Single direct transcription
		res, err := sendGroqWhisperRequest(ctx, apiKey, model, language, customPrompt, tempAudioPath)
		if err != nil {
			return nil, err
		}
		combinedResult.Language = res.Language
		combinedResult.Duration = res.Duration
		combinedResult.Text = strings.TrimSpace(res.Text)

		for _, seg := range res.Segments {
			allSegments = append(allSegments, Segment{
				ID:    seg.ID,
				Start: seg.Start,
				End:   seg.End,
				Text:  strings.TrimSpace(seg.Text),
			})
		}
	} else {
		// Oversized audio: split into chunks
		totalDuration, durErr := getMediaDuration(ctx, tempAudioPath)
		if durErr != nil || totalDuration <= 0 {
			// Fallback estimate by byte size (48kbps = 6000 bytes/sec)
			totalDuration = float64(audioStat.Size()) / 6000.0
		}

		numChunks := int((totalDuration / ChunkDurationSeconds) + 1)
		for i := 0; i < numChunks; i++ {
			startSec := float64(i * ChunkDurationSeconds)
			if startSec >= totalDuration {
				break
			}
			chunkAudioPath := filepath.Join(tempDir, fmt.Sprintf("orbit_chunk_%d_%d.mp3", time.Now().UnixNano(), i))

			// Cut segment with ffmpeg
			cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", fmt.Sprintf("%.2f", startSec), "-t", fmt.Sprintf("%d", ChunkDurationSeconds), "-i", tempAudioPath, "-acodec", "copy", chunkAudioPath)
			if cutErr := cmd.Run(); cutErr != nil {
				os.Remove(chunkAudioPath)
				continue
			}

			chunkRes, chunkErr := sendGroqWhisperRequest(ctx, apiKey, model, language, customPrompt, chunkAudioPath)
			os.Remove(chunkAudioPath)
			if chunkErr != nil {
				return nil, fmt.Errorf("error al transcribir segmento %d: %w", i+1, chunkErr)
			}

			if combinedResult.Language == "" {
				combinedResult.Language = chunkRes.Language
			}
			combinedResult.Duration += chunkRes.Duration

			if fullTextBuilder.Len() > 0 {
				fullTextBuilder.WriteString(" ")
			}
			fullTextBuilder.WriteString(strings.TrimSpace(chunkRes.Text))

			for _, seg := range chunkRes.Segments {
				allSegments = append(allSegments, Segment{
					ID:    len(allSegments),
					Start: seg.Start + startSec,
					End:   seg.End + startSec,
					Text:  strings.TrimSpace(seg.Text),
				})
			}
		}
		combinedResult.Text = fullTextBuilder.String()
	}

	combinedResult.Segments = allSegments

	// Step 3: Format output
	if withTimestamps && len(allSegments) > 0 {
		var formattedBuilder strings.Builder
		for _, seg := range allSegments {
			formattedBuilder.WriteString(fmt.Sprintf("[%s - %s] %s\n", formatTimestamp(seg.Start), formatTimestamp(seg.End), seg.Text))
		}
		combinedResult.FormattedText = strings.TrimSpace(formattedBuilder.String())
	} else {
		combinedResult.FormattedText = combinedResult.Text
	}

	return &combinedResult, nil
}
