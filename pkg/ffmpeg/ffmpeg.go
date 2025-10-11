package ffmpeg

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type FFProbeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

type Resolution struct {
	Name     string // e.g. "1080p"
	Width    int
	Height   int
	Bitrate  string // e.g. "5000k"
	RepIndex int    // FFmpeg representation ID: 0, 1, 2, ...
}

func DecideDownscale(width, height int) []Resolution {
	var res []Resolution
	var index = 0

	appendIf := func(name string, w, h int, bitrate string) {
		if width >= w && height >= h {
			res = append(res, Resolution{name, w, h, bitrate, index})
			index++
		}
	}

	appendIf("1080p", 1920, 1080, "5000k")
	appendIf("720p", 1280, 720, "3000k")
	appendIf("480p", 854, 480, "1500k")
	appendIf("360p", 640, 360, "800k")
	appendIf("240p", 426, 240, "400k")

	return res
}

func TranscodeResolution(inputPath, outputPath string, resolution int) error {
	var scaleFilter string
	switch resolution {
	case 1080:
		scaleFilter = "scale=-2:1080"
	case 720:
		scaleFilter = "scale=-2:720"
	case 480:
		scaleFilter = "scale=-2:480"
	case 360:
		scaleFilter = "scale=-2:360"
	case 240:
		scaleFilter = "scale=-2:240"
	default:
		return fmt.Errorf("unsupported resolution: %s", resolution)
	}

	cmd := exec.Command("ffmpeg",
		"-i", fmt.Sprintf("%s", inputPath),
		"-vf", scaleFilter,
		"-c:a", "aac",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		outputPath,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running ffmpeg for %s: %v", resolution, cmd.Args)
	return cmd.Run()
}

func GetVideoResolution(url string) (width int, height int, err error) {
	cmd := exec.Command("ffprobe",
		"-loglevel", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe error: %v, output: %s", err, string(out))
	}

	var result FFProbeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, 0, err
	}

	if len(result.Streams) == 0 {
		return 0, 0, fmt.Errorf("no video stream found")
	}

	width = result.Streams[0].Width
	height = result.Streams[0].Height
	return width, height, nil
}

func ProcessTranscodeJob(bucket, videoID, sourceURL string, scales []Resolution) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(scales))

	wg.Add(1)
	go func(scales []Resolution) {
		defer wg.Done()
		// ctx := context.Background()

		tmpDir := filepath.Join(os.TempDir(), videoID)
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			errs <- fmt.Errorf("create temp dir failed: %w", err)
			return
		}

		// Transcode
		if err := TranscodeToDASH(videoID, sourceURL, tmpDir, scales); err != nil {
			errs <- fmt.Errorf("transcode %v failed: %w", scales, err)
			return
		}

		// Optional: cleanup
		_ = os.RemoveAll(tmpDir)
	}(scales)

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err // kembalikan error pertama
		}
	}
	return nil
}

func TranscodeToDASH(videoID, sourceURL, outputPath string, resolutions []Resolution) error {
	var filterSplit []string
	var filterScales []string
	var mapArgs []string
	var codecArgs []string

	for i, res := range resolutions {
		filterSplit = append(filterSplit, fmt.Sprintf("[v%d]", i))
		filterScales = append(filterScales,
			fmt.Sprintf("[v%d]scale=%d:%d[v%do]", i, res.Width, res.Height, i),
		)

		// Map video dan audio
		mapArgs = append(mapArgs,
			"-map", fmt.Sprintf("[v%do]", i),
			"-map", "0:a",
		)

		// Codec & bitrate per stream
		codecArgs = append(codecArgs,
			"-c:v:"+strconv.Itoa(i), "libx264",
			"-b:v:"+strconv.Itoa(i), res.Bitrate,
			"-s:v:"+strconv.Itoa(i), fmt.Sprintf("%dx%d", res.Width, res.Height),
			"-representation_id", res.Name,
		)
	}

	filterComplex := fmt.Sprintf("[0:v]split=%d%s;%s",
		len(resolutions),
		strings.Join(filterSplit, ""),
		strings.Join(filterScales, ";"),
	)

	args := []string{
		"-loglevel", "error",
		"-i", sourceURL,
		"-filter_complex", filterComplex,
	}

	args = append(args, mapArgs...)
	args = append(args, codecArgs...)
	args = append(args,
		"-c:a", "aac",
		"-ac", "2",
		"-b:a", "128k",
		"-bf", "1",
		"-keyint_min", "48",
		"-g", "48",
		"-sc_threshold", "0",
		"-use_template", "1",
		"-use_timeline", "1",
		"-init_seg_name", "init-$RepresentationID$.mp4",
		"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
		"-seg_duration", "2",
		"-adaptation_sets", "id=0,streams=v id=1,streams=a",
		"-f", "dash",
		filepath.Join(outputPath, "manifest.mpd"),
	)

	zap.L().Info("ffmpeg: trancoding", zap.Strings("args", args))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func ExtractAudioWithFFmpeg(inputPath, outputPath string) error {
	args := []string{
		"-i", inputPath, // input video
		"-vn",         // no video
		"-c:a", "aac", // encode audio to AAC
		"-b:a", "128k", // bitrate
		"-ac", "2", // stereo
		"-y", // overwrite output
		filepath.Join(outputPath, "audio.m4a"),
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Running FFmpeg for audio:", cmd.String())
	return cmd.Run()
}

func TranscodeMultipleResolutions(inputPath, outputDir string, resolutions []Resolution) error {

	// Filter split dan scale
	var splitLabels, scaleFilters, mapArgs []string
	for i, r := range resolutions {
		splitLabels = append(splitLabels, fmt.Sprintf("[v%d]", i))
		scaleFilters = append(scaleFilters, fmt.Sprintf("[v%d]scale=%d:%d[v%do]", i, r.Width, r.Height, i))

		outFile := filepath.Join(outputDir, fmt.Sprintf("video_%s.mp4", r.Name))
		mapArgs = append(mapArgs,
			"-map", fmt.Sprintf("[v%do]", i),
			"-c:v:"+fmt.Sprint(i), "libx264",
			"-b:v:"+fmt.Sprint(i), r.Bitrate,
			outFile,
		)
	}

	filterComplex := fmt.Sprintf("[0:v]split=%d%s;%s", len(resolutions), join(splitLabels, ""), join(scaleFilters, ";"))

	args := []string{"-i", inputPath, "-filter_complex", filterComplex}
	args = append(args, mapArgs...)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func join(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}

func RunShakaPackagerFromResolutions(
	videoBasePath string, // folder path tempat video_720.mp4, dst.
	outputDir string, // folder hasil output
	resolutions []Resolution,
) error {
	var args []string

	// DRM Label untuk video dan audio (opsional)
	const videoLabel = "SD"
	const audioLabel = "AUDIO"

	// Tambah video input segmented
	for _, res := range resolutions {
		name := res.Name // contoh: "240p", "480p", dst

		videoInput := filepath.Join(outputDir, fmt.Sprintf("video_%s.mp4", name))
		initSegment := filepath.Join(outputDir, fmt.Sprintf("init-%s.mp4", name))
		segmentTemplate := filepath.Join(outputDir, fmt.Sprintf("chunk-%s-$Number$.m4s", name))

		arg := fmt.Sprintf(
			"input=%s,stream=video,init_segment=%s,segment_template=%s,drm_label=%s",
			videoInput, initSegment, segmentTemplate, videoLabel,
		)
		args = append(args, arg)
	}

	// Tambah audio input segmented
	audioInput := filepath.Join(outputDir, "audio.m4a")
	initAudio := filepath.Join(outputDir, "init-audio.mp4")
	audioSegment := filepath.Join(outputDir, "chunk-audio-$Number$.m4s")

	audioArg := fmt.Sprintf(
		"input=%s,stream=audio,init_segment=%s,segment_template=%s,drm_label=%s",
		audioInput, initAudio, audioSegment, audioLabel,
	)
	args = append(args, audioArg)

	// DRM
	// args = append(args, "--enable_raw_key_encryption")
	// args = append(args, "--keys",
	// 	fmt.Sprintf("label=SD:key_id=%s,key=%s"),
	// 	fmt.Sprintf("label=AUDIO:key_id=%s,key=%s"))

	// Output manifest.mpd
	mpdOutput := filepath.Join(outputDir, "manifest.mpd")
	args = append(args, "--generate_static_live_mpd")
	args = append(args, fmt.Sprintf("--mpd_output=%s", mpdOutput))

	// Logging
	zap.L().Info("shaka packager", zap.Strings("args", args))

	// Execute command
	cmd := exec.Command("packager", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Running:", strings.Join(cmd.Args, " "))
	return cmd.Run()
}
