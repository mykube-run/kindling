package ffmpeg

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

const (
	TestUrlVideo          = "https://static.tuputech.com/lts/image/original/lts-1/LTS-Bucket-UnitTest/video/dancing.mp4"
	TestUrlStream         = "rtmp://live.dlddw.xyz/test/test?auth_key=1681439916-0-0-ca97719789e3f63f8432a804a73309ea"
	TestStreamSEI         = "rtmp://ali.dlddw.xyz:1935/live/test"
	TestUrlSpeech         = "https://static.tuputech.com/lts/image/original/lts-1/LTS-Bucket-UnitTest/audio/terror.mp3"
	TestUrlInvalidUrl     = "static.tuputech.com/lts/image/original/lts-1/LTS-Bucket-UnitTest/video/invalid-url.mp4"
	TestUrlSpeechAndVideo = "https://unit-test-resources-1252177232.cos.ap-shanghai.myqcloud.com/politics.mp4"
	TestUrlOnlyVideo      = "https://unit-test-resources-1252177232.cos.ap-shanghai.myqcloud.com/video/pure.mp4"
	TestUrlExpired        = "https://httpbin.org/status/403"
	TestUrlNotFound       = "https://httpbin.org/status/404"
	TestUrl5XX            = "https://httpbin.org/status/502"
	TestUrlInvalidData    = "https://httpbin.org/base64/SFRUUEJJTiBpcyBhd2Vzb21l"
	TestUrlInvalidData2   = "https://static.tuputech.com/lts/image/original/lts-1/LTS-Bucket-UnitTest/video/invalid-data.mp4"
	TestUrlTimeout        = "https://google.com/video.mp4" // Only run this test behind the GFW
	TestUrlInvalidHost    = "https://asfhalfa8isuf4hkajfndsafaksfhasfasf.com/video.mp4"
)

func TestParseSliceCommand(t *testing.T) {
	opt := NewDefaultSliceOptions()
	opt.Uri = "rtmp://sample.com/stream"
	opt.OutputDir = "/tmp/ffmpeg-test"
	cmd := ParseSliceCommand(opt)
	if cmd != "ffmpeg -hide_banner -loglevel warning -reconnect 1 -reconnect_on_network_error 1 -reconnect_streamed 1 -reconnect_delay_max 2 -i 'rtmp://sample.com/stream' -vn -c:a pcm_s16le -ar 16000 -ac 1 -f segment -segment_time 10 /tmp/ffmpeg-test/%012d.wav -y" {
		t.Fatalf("unexpected command: %v", cmd)
	}

	opt.DecodeSEI = true
	opt.SEIOutputDir = "/tmp/sei-test"
	opt.SEIFragmentSuffix = "flv"
	cmd = ParseSliceCommand(opt)
	if cmd != "ffmpeg -hide_banner -loglevel warning -reconnect 1 -reconnect_on_network_error 1 -reconnect_streamed 1 -reconnect_delay_max 2 -i 'rtmp://sample.com/stream' -c:a pcm_s16le -ar 16000 -ac 1 -f segment -segment_time 10 /tmp/ffmpeg-test/%012d.wav -c copy -f segment -segment_time 10 /tmp/sei-test/%012d.flv -y" {
		t.Fatalf("unexpected command: %v", cmd)
	}
}

func TestParseCaptureCommand(t *testing.T) {

	opt := &CaptureOptions{
		CommonOptions: CommonOptions{
			Suffix:   "jpeg",
			LogLevel: "warning",
		},
		Rate: 0.5,
		Size: "1024x576",
	}
	opt.Uri = "rtmp://sample.com/stream"
	opt.OutputDir = "/tmp/ffmpeg-test"
	cmd := ParseCaptureCommand(opt)
	if cmd != "ffmpeg -hide_banner -loglevel warning -reconnect 1 -reconnect_on_network_error 1 -reconnect_streamed 1 -reconnect_delay_max 2 -i 'rtmp://sample.com/stream' -vf 'select=isnan(prev_selected_t)+gte(t-prev_selected_t\\,2)' -r 0.5 -f image2 -qscale:v 1 -qmin 1 -s 1024x576 /tmp/ffmpeg-test/%012d.jpeg -y" {
		t.Fatalf("unexpected command: %v", cmd)
	}
}

func TestCommand_SliceAndCapture(t *testing.T) {

	opt0 := CommonOptions{
		Uri:      TestUrlSpeechAndVideo,
		LogLevel: "warning",
	}

	opt1 := CaptureOptions{
		CommonOptions: CommonOptions{
			Suffix:    "jpeg",
			OutputDir: "/tmp/ffmpeg-test/video",
		},
		Rate: 0.5,
		Size: "1024x576",
	}

	opt2 := SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:    "wav",
			OutputDir: "/tmp/ffmpeg-test/speech",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}

	opt := &SliceAndCaptureOptions{
		CommonOptions:   opt0,
		CaptureOptions:  &opt1,
		SliceOptions:    &opt2,
		HasSpeechStream: true,
		HasImageStream:  true,
	}

	cmd := NewCommand()
	defer cmd.Close()

	err := cmd.SliceAndCapture(opt)
	if err != nil {
		t.Fatalf("should be able to slice and capture, got error: %v", err)
	}

	speechFragment := 0
	images := 0
	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: files: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 30 {
				t.Fatalf("expecting output to be 30, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			switch o.Type {
			case OutputTypeAudioSegment:
				{
					speechFragment++
				}

			case OutputTypeImage:
				{
					images++
				}
			}
			fmt.Printf("[%v]: bytes: %v, suffix: %v, type: %v\n", o.Index, len(o.Content), o.Suffix, o.Type)
		}
	}
	assert.Equal(t, 25, images)
	assert.Equal(t, 5, speechFragment)
}

func TestCommand_SliceAndCaptureIsSametimeOutput(t *testing.T) {

	opt0 := CommonOptions{
		Uri:      TestUrlSpeechAndVideo,
		LogLevel: "warning",
	}

	opt1 := CaptureOptions{
		CommonOptions: CommonOptions{
			Suffix:    "jpeg",
			OutputDir: "/tmp/ffmpeg-test/video",
		},
		Rate: 0.5,
		Size: "1024x576",
	}

	opt2 := SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:    "wav",
			OutputDir: "/tmp/ffmpeg-test/speech",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}

	opt := &SliceAndCaptureOptions{
		CommonOptions:   opt0,
		CaptureOptions:  &opt1,
		SliceOptions:    &opt2,
		HasImageStream:  true,
		HasSpeechStream: true,
	}

	cmd := NewCommand()
	defer cmd.Close()

	err := cmd.SliceAndCapture(opt)
	if err != nil {
		t.Fatalf("should be able to slice and capture, got error: %v", err)
	}

	speechFragment := 0
	images := 0
	res := make([]string, 0)
	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: files: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 30 {
				t.Fatalf("expecting output to be 30, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			switch o.Type {
			case OutputTypeAudioSegment:
				{
					speechFragment++
				}
				res = append(res, "speechFragment")
			case OutputTypeImage:
				{
					images++
				}
				res = append(res, "captured")
			}
			fmt.Printf("[%v]: bytes: %v, suffix: %v, type: %v\n", o.Index, len(o.Content), o.Suffix, o.Type)
		}
	}
	maxConsecutive := maxConsecutiveOccurrences(res)
	fmt.Println(res)
	fmt.Println(maxConsecutive)
	assert.Less(t, maxConsecutive, images/speechFragment*2)
	assert.Equal(t, 25, images)
	assert.Equal(t, 5, speechFragment)
}

func TestCommand_SliceAndCaptureIsSametimeOutput_Stream(t *testing.T) {
	opt0 := CommonOptions{
		Uri:      TestUrlStream,
		LogLevel: "warning",
		IsStream: true,
	}

	opt1 := CaptureOptions{
		CommonOptions: CommonOptions{
			Suffix:    "jpeg",
			OutputDir: "/tmp/ffmpeg-test/video",
		},
		Rate: 0.5,
		Size: "1024x576",
	}

	opt2 := SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:    "wav",
			OutputDir: "/tmp/ffmpeg-test/speech",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}

	opt := &SliceAndCaptureOptions{
		CommonOptions:   opt0,
		CaptureOptions:  &opt1,
		SliceOptions:    &opt2,
		HasSpeechStream: true,
		HasImageStream:  true,
	}

	cmd := NewCommand()
	defer cmd.Close()

	err := cmd.SliceAndCapture(opt)
	if err != nil {
		t.Fatalf("should be able to slice and capture, got error: %v", err)
	}

	speechFragment := 0
	images := 0
	res := make([]string, 0)
	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: files: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			break
		}
		if ok {
			switch o.Type {
			case OutputTypeAudioSegment:
				{
					speechFragment++
				}
				res = append(res, "speechFragment")
			case OutputTypeImage:
				{
					images++
				}
				res = append(res, "captured")
			}
			fmt.Printf("[%v]: bytes: %v, suffix: %v, type: %v\n", o.Index, len(o.Content), o.Suffix, o.Type)
		}
	}
	maxConsecutive := maxConsecutiveOccurrences(res)
	fmt.Println(res)
	fmt.Println(maxConsecutive)
	if speechFragment != 0 {
		assert.Less(t, maxConsecutive, images/speechFragment*2)
	}
}

func TestCommand_SliceAndCapture_OnlyVideo(t *testing.T) {

	opt0 := CommonOptions{
		Uri:      TestUrlOnlyVideo,
		LogLevel: "debug",
	}

	opt1 := CaptureOptions{
		CommonOptions: CommonOptions{
			OutputDir: "/tmp/ffmpeg-test/video",
			Suffix:    "jpeg",
		},
		Rate: 0.5,
		Size: "1024x576",
	}

	opt2 := SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:    "wav",
			OutputDir: "/tmp/ffmpeg-test/speech",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}

	opt := &SliceAndCaptureOptions{
		CommonOptions:   opt0,
		CaptureOptions:  &opt1,
		SliceOptions:    &opt2,
		HasSpeechStream: false,
		HasImageStream:  true,
	}

	cmd := NewCommand()
	defer cmd.Close()

	err := cmd.SliceAndCapture(opt)
	if err != nil {
		t.Fatalf("should be able to slice and capture, got error: %v", err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			assert.Error(t, err, "NO_STREAM")
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: files: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 25 {
				t.Fatalf("expecting output to be 30, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			fmt.Printf("[%v]: bytes: %v, suffix: %v, type: %v, Second: %v, Position: %v\n", o.Index, len(o.Content), o.Suffix, o.Type, o.Second, o.Position)
		}
	}
}

func TestCommand_SliceAndCapture_OnlyAudio(t *testing.T) {

	opt0 := CommonOptions{
		Uri:      TestUrlSpeech,
		LogLevel: "debug",
	}

	opt1 := CaptureOptions{
		CommonOptions: CommonOptions{
			Suffix:    "jpeg",
			OutputDir: "/tmp/ffmpeg-test/video",
		},
		Rate: 0.5,
		Size: "1024x576",
	}

	opt2 := SliceOptions{
		CommonOptions: CommonOptions{
			Suffix:    "wav",
			OutputDir: "/tmp/ffmpeg-test/speech",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  10,
	}

	opt := &SliceAndCaptureOptions{
		CommonOptions:   opt0,
		CaptureOptions:  &opt1,
		SliceOptions:    &opt2,
		HasSpeechStream: true,
		HasImageStream:  false,
	}

	cmd := NewCommand()
	defer cmd.Close()

	err := cmd.SliceAndCapture(opt)
	if err != nil {
		t.Fatalf("should be able to slice and capture, got error: %v", err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			assert.Error(t, err, "NO_STREAM")
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: files: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 1 {
				t.Fatalf("expecting output to be 30, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			fmt.Printf("[%v]: bytes: %v, suffix: %v, type: %v, Second: %v, Position: %v\n", o.Index, len(o.Content), o.Suffix, o.Type, o.Second, o.Position)
		}
	}
}

func TestCommand_ProbeStreams(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	opt := &ProbeOptions{
		Uri:      TestUrlVideo,
		IsStream: false,
		IsFile:   true,
		LogLevel: "error",
	}

	cmd := NewCommand()
	defer cmd.Close()

	st, err := cmd.ProbeStreams(opt)
	if err != nil {
		t.Fatalf("should be able to probe streams, got error: %v", err)
	}
	if _, ok := st.HasVideoStream(); !ok {
		t.Fatalf("expecting video stream, but not found")
	}
	if _, ok := st.HasAudioStream(); !ok {
		t.Fatalf("expecting audio stream, but not found")
	}

	dur, err := st.GetVideoDuration()
	if err != nil {
		t.Fatalf("expecting valid video duration, got error: %v", err)
	}
	if dur == 0 {
		t.Fatalf("expecting valid video duration, got 0")
	}
	log.Info().Float64("duration", dur).Msg("video duration")

	dur, err = st.GetAudioDuration()
	if err != nil {
		t.Fatalf("expecting valid audio duration, got error: %v", err)
	}
	if dur == 0 {
		t.Fatalf("expecting valid audio duration, got 0")
	}
	log.Info().Float64("duration", dur).Msg("audio duration")

	dur, err = st.Format.GetDuration()
	if err != nil {
		t.Fatalf("expecting valid duration from format, got error: %v", err)
	}
	if dur == 0 {
		t.Fatalf("expecting valid duration from format, got 0")
	}
	log.Info().Float64("duration", dur).Msg("duration from format")
}

func TestCommand_ProbeStreams_RetryOnNoStream(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	opt := &ProbeOptions{
		Uri:                TestUrlInvalidData,
		IsStream:           true,
		IsFile:             false,
		LogLevel:           "error",
		RetryStreamOnError: true,
		RetryInterval:      time.Second * 5,
		MaxRetry:           2,
	}

	cmd := NewCommand()
	defer cmd.Close()

	start := time.Now()
	_, err := cmd.ProbeStreams(opt)
	if err == nil {
		t.Fatalf("expecting non-nil error, got nil")
	}
	if time.Now().Sub(start).Seconds() < 10 {
		t.Fatalf("should be retried")
	}
}

func TestCommand_Capture(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	opt := &CaptureOptions{
		CommonOptions: CommonOptions{
			Uri:            TestUrlVideo,
			OutputDir:      "/tmp/ffmpeg-test",
			Suffix:         "jpg",
			MediaId:        "test",
			IsStream:       false,
			IsFile:         false,
			PreserveOutput: false,
			Proxy:          "",
			LogLevel:       "error",
			DockerCommand:  "",
		},
		Rate:      1,
		MaxFrames: 0,
		Size:      "360x640",
		Mode:      CaptureModeByInterval,
		Frame:     1,
		Debug:     false,
	}

	c := ParseCaptureCommand(opt)
	if !strings.Contains(c, "reconnect") {
		t.Fatalf("expecting reconnect in cmd string")
	}

	cmd := NewCommand()
	defer cmd.Close()

	var cnt int64 = 0
	if err := cmd.Capture(opt); err != nil {
		t.Fatal(err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			t.Error(err)
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: captured: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 11 {
				t.Fatalf("expecting output to be 11, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			cnt += 1
			fmt.Printf("[%v]: bytes: %v\n", o.Index, len(o.Content))
		}
	}
}

func TestCommand_Capture_ByFrame(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	opt := &CaptureOptions{
		CommonOptions: CommonOptions{
			Uri:           TestUrlVideo,
			OutputDir:     "/tmp/ffmpeg-test",
			Suffix:        "jpg",
			MediaId:       "test",
			IsStream:      false,
			IsFile:        false,
			Proxy:         "",
			LogLevel:      "error",
			DockerCommand: "",
		},
		Rate:      1,
		MaxFrames: 0,
		Size:      "360x640",
		Mode:      CaptureModeByFrame,
		Frame:     2,
		Debug:     true,
	}

	cmd := NewCommand()
	defer cmd.Close()
	var cnt int64 = 0
	if err := cmd.Capture(opt); err != nil {
		t.Fatal(err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			t.Error(err)
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: captured: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 13 {
				t.Fatalf("expecting output to be 13, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			cnt += 1
			fmt.Printf("[%v]: bytes: %v\n", o.Index, len(o.Content))
		}
	}
}

func TestCommand_Capture_Stream(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	opt := &CaptureOptions{
		CommonOptions: CommonOptions{
			Uri:           TestUrlVideo,
			OutputDir:     "/tmp/ffmpeg-test",
			Suffix:        "jpg",
			MediaId:       "test",
			IsStream:      true,
			IsFile:        false,
			Proxy:         "",
			LogLevel:      "error",
			DockerCommand: "",
			IOTimeout:     120,
		},
		Rate:      1,
		MaxFrames: 0,
		Size:      "360x640",
		Mode:      CaptureModeByInterval,
		Frame:     1,
		Debug:     false,
	}

	cmd := NewCommand()
	defer cmd.Close()

	var cnt int64 = 0
	if err := cmd.Capture(opt); err != nil {
		t.Fatal(err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			t.Error(err)
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: captured: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			if cmd.Stats().Output != 11 {
				t.Fatalf("expecting output to be 11, got %v", cmd.Stats().Output)
			}
			break
		}
		if ok {
			cnt += 1
			fmt.Printf("[%v]: bytes: %v\n", o.Index, len(o.Content))
		}
	}
}

func TestCommand_Slice(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	opt := &SliceOptions{
		CommonOptions: CommonOptions{
			Uri:           TestUrlSpeech,
			OutputDir:     "/tmp/ffmpeg-test",
			Suffix:        "wav",
			MediaId:       "test",
			IsStream:      true,
			IsFile:        false,
			DecodeSEI:     false,
			Proxy:         "",
			LogLevel:      "error",
			DockerCommand: "",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  2,
	}

	cmd := NewCommand()
	defer cmd.Close()

	var cnt int64 = 0
	if err := cmd.Slice(opt); err != nil {
		t.Fatal(err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			t.Error(err)
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: sliced: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			break
		}
		if ok {
			cnt += 1
			fmt.Printf("[%v]: bytes: %v, is last: %v\n", o.Index, len(o.Content), o.Last)
		}
	}
}

func TestCommand_SliceWithSEI(t *testing.T) {
	opt := &SliceOptions{
		CommonOptions: CommonOptions{
			Uri:               TestStreamSEI,
			OutputDir:         "/tmp/ffmpeg-test",
			Suffix:            "wav",
			SEIOutputDir:      "/tmp/sei-test",
			SEIFragmentSuffix: "flv",
			MediaId:           "test",
			IsStream:          true,
			IsFile:            false,
			DecodeSEI:         true,
			Proxy:             "",
			LogLevel:          "error",
			DockerCommand:     "",
		},
		DisableVideo:      true,
		Coding:            "pcm_s16le",
		SamplingFrequency: 16000,
		Channels:          1,
		Format:            "segment",
		FragmentDuration:  20,
	}

	cmd := NewCommand()
	defer cmd.Close()

	var cnt2 int64 = 0
	if err := cmd.Slice(opt); err != nil {
		t.Fatal(err)
	}

	for {
		o, err, ok, finished := cmd.ReadOutput()
		if err != nil {
			t.Error(err)
			break
		}
		if finished {
			fmt.Println("closed")
			fmt.Printf("stats: sliced: %v, bytes: %v\n", cmd.stats.Output, cmd.stats.Bytes)
			break
		}
		if ok {
			cnt2 += 1
			fmt.Printf("[%v]: bytes: %v, is last: %v\n", o.Index, len(o.Content), o.Last)
			if o.SEIInfo != nil {
				fmt.Println(o.SEIInfo)
			}
		}
	}
}

func TestCommand_Error(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	opt := &CaptureOptions{
		CommonOptions: CommonOptions{
			Uri:           TestUrlVideo,
			OutputDir:     "/tmp/ffmpeg-test",
			Suffix:        "jpg",
			MediaId:       "test",
			IsStream:      false,
			IsFile:        false,
			Proxy:         "",
			LogLevel:      "info",
			DockerCommand: "",
		},
		Rate:      1,
		MaxFrames: 0,
		Size:      "1024x768",
	}

	t.Run("ErrInvalidUrl", func(t *testing.T) {
		// ErrInvalidUrl
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlInvalidUrl
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrInvalidUrl {
						t.Fatalf("expecting ErrInvalidUrl, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("ErrUrlExpiredOrForbidden", func(t *testing.T) {
		// ErrUrlExpiredOrForbidden
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlExpired
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrUrlExpiredOrForbidden {
						t.Fatalf("expecting ErrUrlExpiredOrForbidden, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("ErrUrlNotFound", func(t *testing.T) {
		// ErrUrlNotFound
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlNotFound
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrUrlNotFound {
						t.Fatalf("expecting ErrUrlNotFound, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("ErrMediaServerError", func(t *testing.T) {
		// ErrMediaServerError
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrl5XX
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrMediaServerError {
						t.Fatalf("expecting ErrMediaServerError, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("TestUrlInvalidData", func(t *testing.T) {
		// ErrInvalidData
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlInvalidData
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrInvalidData {
						t.Fatalf("expecting ErrInvalidData, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("TestUrlInvalidData-2", func(t *testing.T) {
		// ErrInvalidData
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlInvalidData2
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrInvalidData {
						t.Fatalf("expecting ErrInvalidData, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("ErrResolveHostFailed", func(t *testing.T) {
		// ErrResolveHostFailed
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlInvalidHost
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrResolveHostFailed {
						t.Fatalf("expecting ErrResolveHostFailed, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

	t.Run("ErrConnectionTimeout", func(t *testing.T) {
		// ErrConnectionTimeout
		// Only run this test behind the GFW
		{
			cmd := NewCommand()
			defer cmd.Close()
			opt.Uri = TestUrlTimeout
			if err := cmd.Capture(opt); err != nil {
				t.Fatal(err)
			}

			for {
				_, err, ok, finished := cmd.ReadOutput()
				if err != nil {
					if err != ErrConnectionTimeout {
						t.Fatalf("expecting ErrConnectionTimeout, got %v", err)
					}
					break
				}
				if finished {
					t.Fatalf("expecting an error, should not finish")
				}
				if ok {
					t.Fatalf("expecting an error, should not receive an output")
				}
			}
		}
	})

}

func maxConsecutiveOccurrences(arr []string) int {
	maxCount := 1
	currCount := 1

	for i := 1; i < len(arr); i++ {
		if arr[i] == arr[i-1] {
			currCount++
		} else {
			if currCount > maxCount {
				maxCount = currCount
			}
			currCount = 1
		}
	}

	if currCount > maxCount {
		maxCount = currCount
	}
	return maxCount
}
