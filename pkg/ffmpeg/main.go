package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	OutputTypeImage        = 1 // Output as image
	OutputTypeAudioSegment = 2 // Output as audio segment
)

const (
	CaptureModeByInterval = iota // Capture by duration (every n seconds), default value
	CaptureModeByFrame           // Capture by every n frame
)

const (
	DefaultIOTimeout = 2 // Default to 2 seconds
)

// CommonOptions common options for FFmpeg command
type CommonOptions struct {
	Uri               string // Video, speech, stream url or file path
	OutputDir         string // Output directory
	Suffix            string // Output file suffix, e.g. jpg, png, wav
	SEIOutputDir      string // SEI fragment output directory
	SEIFragmentSuffix string // SEI fragment suffix
	MediaId           string // Media id
	IsStream          bool   // Whether the media is a stream
	IsFile            bool   // Whether the media is a local file
	DecodeSEI         bool   // Whether to decode SEI
	PreserveOutput    bool   // Whether to preserve outputs (not deleting output), not recommended for production usage
	Proxy             string // HTTP proxy
	LogLevel          string // FFmpeg log level
	DockerCommand     string // FFmpeg docker command
	IOTimeout         int    // Timeout for FFmpeg IO operations in seconds

	options
}

// options Internal use of options
type options struct {
	Suffixes         []string // eg: ["jpeg", "wav"]
	SliceAndCapture  bool     // Is turn on slicing and screenshot pictures at the same time
	SliceOutputDir   string   // speech output dir
	CaptureOutputDir string   // image output dir
	HasSpeech        bool     // image output dir
	HasVideo         bool     // image output dir
}

// HttpProxy returns a valid HTTP proxy address prefixed with scheme
func (opt *CommonOptions) HttpProxy() string {
	if strings.HasPrefix(opt.Proxy, "http") {
		return opt.Proxy
	}
	return "http://" + opt.Proxy
}

// GetLogLevel returns a valid log level
func (opt *CommonOptions) GetLogLevel() string {
	switch opt.LogLevel {
	case "quiet", "panic", "fatal", "error", "warning", "info", "verbose", "debug", "trace":
		return opt.LogLevel
	}
	return "error"
}

// GetIOTimeout returns a valid IOTimeout value default to DefaultIOTimeout
func (opt *CommonOptions) GetIOTimeout() int {
	if opt.IOTimeout <= 0 {
		return DefaultIOTimeout
	}
	return opt.IOTimeout
}

// CaptureOptions options for capturing images
type CaptureOptions struct {
	CommonOptions
	Rate float32 // Frame capture rate, one frame per 1/rate second, e.g. 0.5
	Size string  // Captured image size in the form of <width>x<height>, e.g. 1024x768

	// The following options have default value
	MaxFrames int  // Maximum capture number, default to 0 (no limit)
	Debug     bool // Enable debug mode (print frame number & time point on captured images)
	Mode      int  // Capture mode, default to CaptureModeByInterval
	Frame     int  // Capture every n frame, available under CaptureModeByFrame
}

// SliceOptions options for slicing audio segments
type SliceOptions struct {
	CommonOptions

	// The following options must be set, use NewDefaultSliceOptions to get a pre-defined option
	DisableVideo      bool   // Disable video stream while slicing
	Coding            string // Audio encoding
	SamplingFrequency int    // Audio sampling frequency
	Channels          int    // Audio channels
	Format            string // Audio format
	FragmentDuration  int    // Sliced fragment duration in seconds
}

func NewDefaultSliceOptions() *SliceOptions {
	return &SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:   "wav",
			LogLevel: "warning",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}
}

// Output captured image or sliced audio segment
type Output struct {
	Type         int      // Output file type
	Index        int64    // Output file index
	Content      []byte   // Output file content
	Suffix       string   // Output file suffix
	Last         bool     // Whether output file is the last fragment/image
	LastSliced   bool     // Whether output file is the last fragment/image
	LastCaptured bool     // Whether output file is the last fragment/image
	SEIInfo      []string // SEI info

	// Captured image
	Position int64   // Capture frame position (at n-th second)
	Second   float64 // Capture frame or audio segment segment second
}

// ProbeOptions stream probing options
type ProbeOptions struct {
	Uri                string        // Video, speech, stream url or file path
	IsStream           bool          // Whether the media is a stream
	IsFile             bool          // Whether the media is a local file
	Proxy              string        // HTTP proxy
	RetryStreamOnError bool          // Whether to retry probing stream on error when uri is a stream
	RetryInterval      time.Duration // Interval between retries
	MaxRetry           int           // Number of maximum retries
	LogLevel           string        // FFmpeg log level
	DockerCommand      string        // FFmpeg docker command
}

type SliceAndCaptureOptions struct {
	CommonOptions
	*SliceOptions
	*CaptureOptions
	HasSpeechStream bool
	HasImageStream  bool
}

// HttpProxy returns a valid HTTP proxy address prefixed with scheme
func (opt *ProbeOptions) HttpProxy() string {
	if strings.HasPrefix(opt.Proxy, "http") {
		return opt.Proxy
	}
	return "http://" + opt.Proxy
}

// GetLogLevel returns a valid log level
func (opt *ProbeOptions) GetLogLevel() string {
	switch opt.LogLevel {
	case "quiet", "panic", "fatal", "error", "warning", "info", "verbose", "debug", "trace":
		return opt.LogLevel
	}
	return "error"
}

// OutputStats capture/sliced statistics
type OutputStats struct {
	Start    time.Time // Time started
	End      time.Time // Time ended
	Duration int64     // Process time
	Output   int       // Number of captured images or sliced audio segments
	Bytes    int64     // Number of output file length
}

// StreamInfo media stream info
type StreamInfo struct {
	Streams []Stream `json:"streams"` // Media streams, in the most common cases #0 is video, #1 is audio
	Format  Format   `json:"format"`  // Media format
}

// HasVideoStream returns whether the media has video stream and its stream index
func (st *StreamInfo) HasVideoStream() (int, bool) {
	for idx, v := range st.Streams {
		if v.CodecType == "video" {
			return idx, true
		}
	}
	return 0, false
}

// HasAudioStream returns whether the media has audio stream and its stream index
func (st *StreamInfo) HasAudioStream() (int, bool) {
	for idx, v := range st.Streams {
		if v.CodecType == "audio" {
			return idx, true
		}
	}
	return 0, false
}

// GetVideoDuration tries to acquire media duration from stream, otherwise from format
func (st *StreamInfo) GetVideoDuration() (float64, error) {
	idx, ok := st.HasVideoStream()
	if !ok {
		return 0, fmt.Errorf("file has no video stream")
	}
	dur, err := st.Streams[idx].GetDuration()
	if err != nil {
		dur, err = st.Format.GetDuration()
	}
	return dur, err
}

// GetAudioDuration tries to acquire media duration from stream, otherwise from format
func (st *StreamInfo) GetAudioDuration() (float64, error) {
	idx, ok := st.HasAudioStream()
	if !ok {
		return 0, fmt.Errorf("file has no audio stream")
	}
	dur, err := st.Streams[idx].GetDuration()
	if err != nil {
		dur, err = st.Format.GetDuration()
	}
	return dur, err
}

// Stream the stream info
type Stream struct {
	Index              int    `json:"index"`
	CodecName          string `json:"codec_name"`
	CodecType          string `json:"codec_type"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	StartTime          string `json:"start_time"`
	Duration           string `json:"duration"`
	AvgFrameRate       string `json:"avg_frame_rate"`
	Frames             string `json:"nb_frames"`
	SampleRate         string `json:"sample_rate"`
}

// GetDuration gets stream's duration
func (s *Stream) GetDuration() (float64, error) {
	return strconv.ParseFloat(s.Duration, 0)
}

// GetFrames gets stream's number of frames
func (s *Stream) GetFrames() (int64, error) {
	return strconv.ParseInt(s.Frames, 10, 0)
}

// GetFrameRate gets stream's frame rate
func (s *Stream) GetFrameRate() (float64, error) {
	spl := strings.Split(s.AvgFrameRate, "/")
	if len(spl) != 2 {
		return 0, fmt.Errorf("invalid avg_frame_rate: %v", s.AvgFrameRate)
	}
	if spl[1] == "0" {
		return 0, fmt.Errorf("invalid avg_frame_rate (denominator = 0): %v", s.AvgFrameRate)
	}
	num, err := strconv.ParseInt(spl[0], 10, 0)
	if err != nil {
		return 0, fmt.Errorf("error parsing numerator: %v, avg_frame_rate: %v", err, s.AvgFrameRate)
	}
	den, err := strconv.ParseInt(spl[1], 10, 0)
	if err != nil {
		return 0, fmt.Errorf("error parsing denominator: %v, avg_frame_rate: %v", err, s.AvgFrameRate)
	}
	return float64(num) / float64(den), nil
}

// GetStartTime gets stream's start time
func (s *Stream) GetStartTime() (float64, error) {
	return strconv.ParseFloat(s.StartTime, 0)
}

// Format media format
type Format struct {
	FormatName string `json:"format_name"`
	FileName   string `json:"filename"`
	StartTime  string `json:"start_time"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
	ProbeScore int    `json:"probe_score"`
}

// GetDuration gets media duration
func (f *Format) GetDuration() (float64, error) {
	return strconv.ParseFloat(f.Duration, 0)
}

// GetSize gets media file size in bytes
func (f *Format) GetSize() (int64, error) {
	return strconv.ParseInt(f.Size, 10, 0)
}
