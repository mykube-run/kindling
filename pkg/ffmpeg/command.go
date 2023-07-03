package ffmpeg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	llq "github.com/emirpasic/gods/queues/linkedlistqueue"
	"github.com/mykube-run/kindling/pkg/retry"
	"github.com/mykube-run/kindling/pkg/utils"
	"github.com/rs/zerolog/log"
	"io"
	"io/fs"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var SEIRegex, _ = regexp.Compile(`{".+([0-9]|}|]|")}`)

const (
	space  = " "
	common = "-hide_banner"
	// Reconnect options.
	// NOTE:
	//		- Some reconnect options are not supported by FFmpeg v3
	//		- The reconnect_on_network_error option is not supported by FFmpeg v4.3
	//		- The reconnect_at_eof may cause endless retry, must not be added here
	reconnect = "-reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 2"
	// Timeout for IO operations (in microseconds)
	rwTimeout = "-rw_timeout %v"
	// filterInterval instructs FFmpeg to capture frames every n seconds, note that there is a pkt_duration_time mistake, meaning
	// captured frames may not be a fixed time point
	// See: https://ffmpeg.org/ffmpeg-all.html#select_002c-aselect
	// NOTE:
	//   	- By using select filter, we're able to filter frames by key frame, or picture type (I, B, P)
	filterInterval = "select=isnan(prev_selected_t)+gte(t-prev_selected_t\\,%v)"
	// filterEveryNFrm instructs FFmpeg to capture frames every n
	// See: https://ffmpeg.org/ffmpeg-all.html#select_002c-aselect
	filterEveryNFrm = "select=not(mod(n\\,%v))"
	// filterDebug instructs FFmpeg to print frame number & time on every captured images, used for debug purpose
	// See: https://ffmpeg.org/ffmpeg-all.html#drawtext-1
	// NOTE: fontconfig must be installed on Linux, otherwise this error will occur:
	//		[Parsed_select_0] Setting 'expr' to value 'not(mod(n,5))'
	//		[Parsed_drawtext_1] Setting 'fontsize' to value '45'
	//		[Parsed_drawtext_1] Setting 'fontcolor' to value 'yellow'
	//		[Parsed_drawtext_1] Setting 'text' to value '%{frame_num} %{pts}'
	//		Fontconfig error: Cannot load default config file
	//		[Parsed_drawtext_1] Cannot find a valid font for the family Sans
	//		[AVFilterGraph] Error initializing filter 'drawtext' with args 'fontsize=45:fontcolor=yellow:text=%{frame_num} %{pts}'
	//		Error reinitializing filters!
	//		Failed to inject frame into filter network: No such file or directory
	//		Error while processing the decoded data for stream #0:0
	filterDebug = `drawtext=fontsize=45:fontcolor=yellow:borderw=1:bordercolor=red:text=%{frame_num} %{pts\\:hms}`
	// Stream probing options
	probe = "-show_streams -show_format -of json"
)

type Command struct {
	cmd *exec.Cmd      // The underlying command instance
	q   *llq.Queue     // Output buffer queue
	opt *CommonOptions // Command options
	mod OutputModifier // Output modifier

	// Since FFmpeg may exit in a very short time, judging by finished may cause several captured/sliced files being ignored.
	// By Comparing lastQueued & lastIndex, we are able to know whether all files are enqueued
	lastQueued       int64       // The newest file index that was enqueued
	lastIndex        int64       // 为了兼容之前单个处理的，这里先不做更改
	lastCaptureIndex int64       // 截帧最后一个下标
	lastSliceIndex   int64       // 语音切片最后一个片段的下标
	existingFileC    chan string // existing file name channel

	closed        bool      // If the Command is closed, caller shall not read from output queue anymore
	started       bool      // Whether FFmpeg process is started, will be set to true after successfully called cmd.Start
	finished      bool      // Whether FFmpeg process is finished, will only be set to true when FFmpeg exit with code 0
	ffmpegExit    bool      // ffmpeg进程是否退出
	finishCapture bool      // Whether FFmpeg process is finished, will only be set to true when FFmpeg exit with code 0
	finishedSlice bool      // Whether FFmpeg process is finished, will only be set to true when FFmpeg exit with code 0
	finishedAt    time.Time // The time when FFmpeg is finished
	err           error     // The error that FFmpeg returned - parsed error for known issues, otherwise "exit status code - message", e.g.: exit status 1 - HTTP 404...

	stats OutputStats // Output statistics
}

type OutputModifier func(*Output)

// NewCommand creates a new FFmpeg command instance
func NewCommand() *Command {
	c := &Command{
		cmd:              nil,
		q:                llq.New(),
		lastQueued:       0,
		lastIndex:        math.MaxInt64,
		lastCaptureIndex: math.MaxInt64,
		lastSliceIndex:   math.MaxInt64,
		existingFileC:    make(chan string),
		closed:           false,
		started:          false,
		finished:         false,
	}
	return c
}

// Capture captures specified input media into images
// NOTE:
//  1. The output directory MUST BE EMPTY. Command iterates files in output directory to
//     determine the last index after FFmpeg process finishes, any other files may cause Command block (unable to exit)
func (c *Command) Capture(opt *CaptureOptions) error {
	cmd := ParseCaptureCommand(opt)
	fn := func(o *Output) {
		o.Type = OutputTypeImage
		o.Suffix = opt.Suffix
		o.Position = utils.GetImagePosition(o.Index, opt.Rate)
		o.Second = utils.GetImageSecond(o.Index, opt.Rate)
	}
	c.mod = fn
	c.opt = &opt.CommonOptions
	return c.process(&opt.CommonOptions, cmd)
}

// Slice slices specified input media into speech fragments
// NOTE:
//  1. The output directory MUST BE EMPTY. Command iterates files in output directory to
//     determine the last index after FFmpeg process finishes, any other files may cause Command block (unable to exit)
func (c *Command) Slice(opt *SliceOptions) error {
	/* Work around: FFmpeg slices speech fragments starting from 0, with zero we may lose the first fragment event */
	c.lastQueued = -1
	cmd := ParseSliceCommand(opt)
	fn := func(o *Output) {
		o.Type = OutputTypeAudioSegment
		o.Suffix = opt.Suffix
		o.Second = utils.GetSegmentStart(o.Index, opt.FragmentDuration)
	}
	c.opt = &opt.CommonOptions
	if opt.DecodeSEI {
		opt.CommonOptions.DecodeSEI = true
		opt.CommonOptions.SEIOutputDir = opt.SEIOutputDir
		opt.CommonOptions.SEIFragmentSuffix = opt.SEIFragmentSuffix
	}
	c.mod = fn
	return c.process(&opt.CommonOptions, cmd)
}

// SliceAndCapture slices specified input media into speech fragments and captures specified input media into images
// NOTE:
//  1. The output directory MUST BE EMPTY. Command iterates files in output directory to
//     determine the last index after FFmpeg process finishes, any other files may cause Command block (unable to exit)
//  2. The input source must have both video and voice streams
func (c *Command) SliceAndCapture(opt *SliceAndCaptureOptions) error {
	/* Work around: FFmpeg slices speech fragments starting from 0, with zero we may lose the first fragment event */
	c.lastQueued = -1
	c.opt = &opt.CommonOptions
	if opt.DecodeSEI {
		opt.CommonOptions.DecodeSEI = true
		opt.CommonOptions.SEIOutputDir = opt.SEIOutputDir
		opt.CommonOptions.SEIFragmentSuffix = opt.SEIFragmentSuffix
	}
	opt.CommonOptions.SliceAndCapture = true
	// 切片和截帧参数必须有一个
	if opt.SliceOptions == nil && opt.CaptureOptions == nil {
		return fmt.Errorf("SliceOptions CaptureOptions cannot be nil in the same time")
	}
	// 如果有视频流的话，去校验语音参数
	if opt.HasImageStream {
		if opt.CaptureOptions != nil {
			if opt.CaptureOptions.Suffix == "" || opt.CaptureOptions.OutputDir == "" {
				return fmt.Errorf("CaptureOptions.Suffix and CaptureOptions.OutputDir must not be empty")
			}
			opt.CommonOptions.CaptureOutputDir = opt.CaptureOptions.CommonOptions.OutputDir
			opt.CommonOptions.HasVideo = true
		} else {
			return fmt.Errorf("CaptureOptions cannot be nil")
		}
	}
	// 如果有语音流的话，去校验语音参数
	if opt.HasSpeechStream {
		if opt.SliceOptions != nil {
			if opt.SliceOptions.Suffix == "" || opt.SliceOptions.OutputDir == "" {
				return fmt.Errorf("SliceOptions.Suffix and SliceOptions.OutputDir must not be empty")
			}
			opt.CommonOptions.SliceOutputDir = opt.SliceOptions.CommonOptions.OutputDir
			opt.CommonOptions.HasSpeech = true
		} else {
			return fmt.Errorf("SliceOptions cannot be nil")
		}
	}

	cmd := ParseSliceAndCaptureCommand(opt)
	fn := func(o *Output) {
		switch o.Suffix {
		case opt.SliceOptions.Suffix:
			{
				o.Type = OutputTypeAudioSegment
				o.Second = utils.GetSegmentStart(o.Index, opt.FragmentDuration)
			}
		case opt.CaptureOptions.Suffix:
			{
				o.Type = OutputTypeImage
				o.Position = utils.GetImagePosition(o.Index, opt.Rate)
				o.Second = utils.GetImageSecond(o.Index, opt.Rate)
			}
		}
	}
	c.mod = fn
	return c.process(&opt.CommonOptions, cmd)
}

// ReadOutput reads output from the underlying queue, also indicates whether all output are read.
// NOTE: Must break on finished and error in a for loop, e.g.:
//
//	for {
//			o, err, ok, finished := cmd.ReadOutput()
//			// 1. Check whether there is an error
//			if err != nil {
//				fmt.Println(err)
//				break
//			}
//			// 2. Check whether FFmpeg is finished
//			if finished {
//				fmt.Println("finished")
//				break
//			}
//			// 3. Check whether an output file is read
//			if ok {
//				fmt.Printf("[%v]: bytes: %v\n", o.Index, len(o.Content))
//			}
//	}
func (c *Command) ReadOutput() (o *Output, err error, ok bool, finished bool) {
	// FFmpeg process finished and all output are read, safe to stop reading output
	if c.IsFinished() && c.q.Empty() {
		return nil, nil, false, true
	}
	// Try to dequeue output
	if v, ok1 := c.q.Dequeue(); ok1 {
		return v.(*Output), nil, true, false
	}
	// Seems nothing left in queue, see if there is an error
	if err = c.Error(); err != nil {
		return nil, err, false, true
	}
	return nil, nil, false, false
}

// ProbeStreams probes media stream info
// NOTE:
//  1. Will retry once on connection timeout for both file and stream
//  2. Will retry several times when RetryStreamOnError is enabled for stream
func (c *Command) ProbeStreams(opt *ProbeOptions) (st *StreamInfo, err error) {
	if opt.IsStream && opt.RetryStreamOnError {
		fn := func() error {
			st, err = c.probe(opt)
			return err
		}
		err = retry.WithInterval(fn, opt.MaxRetry, opt.RetryInterval)
		return
	}

	// No retry is required
	return c.probe(opt)
}

func (c *Command) probe(opt *ProbeOptions) (st *StreamInfo, err error) {
	cmd := ParseProbeCommand(opt)
	ow := new(bytes.Buffer) // Stdout writer
	err = c.execwait(cmd, ow)
	if err == ErrConnectionTimeout {
		// Retry on connection timeout, note that here we must reset ow
		// otherwise there may be some obsolete message remaining in ow causing JSON unmarshal error
		ow = new(bytes.Buffer)
		err = c.execwait(cmd, ow)
	}
	if err != nil {
		return
	}

	// Parse stream info
	st = new(StreamInfo)
	err = json.Unmarshal(ow.Bytes(), st)
	return
}

// Stats returns command output statistics
func (c *Command) Stats() OutputStats {
	c.stats.Duration = c.stats.End.Sub(c.stats.Start).Milliseconds()
	if c.stats.Duration < 0 {
		c.stats.Duration = 0
	}
	return c.stats
}

// Close kills FFmpeg process, removes watcher and deletes output directory
func (c *Command) Close() error {
	c.closed = true
	if c.started {
		close(c.existingFileC)
		if !(c.finished || c.err != nil) {
			if err := c.kill(c.cmd); err != nil {
				log.Err(err).Msg("error killing ffmpeg process")
			}
		}
	}

	if c.opt != nil && !c.opt.PreserveOutput {
		if err := os.RemoveAll(c.opt.OutputDir); err != nil {
			log.Err(err).Msg("error deleting output directory")
		}
		if c.opt.SEIOutputDir != "" {
			if err := os.RemoveAll(c.opt.SEIOutputDir); err != nil {
				log.Err(err).Msg("error deleting SEI output directory")
			}
		}
	}

	c.q.Clear()
	return nil
}

// IsFinished testifies whether the command is finished
func (c *Command) IsFinished() bool {
	ok := c.finished /* command process finished */
	return ok
}

// Error returns command error
func (c *Command) Error() error {
	return c.err
}

// IsClosed returns whether the command is closed
func (c *Command) IsClosed() bool {
	return c.closed
}

// LastIndex returns last output index, only available after FFmpeg process finished
func (c *Command) LastIndex() int64 {
	return c.lastIndex
}

// LastCaptureIndex returns last Capture output index, only available after FFmpeg process finished
func (c *Command) LastCaptureIndex() int64 {
	return c.lastCaptureIndex
}

// LastSliceIndex returns last slice output index, only available after FFmpeg process finished
func (c *Command) LastSliceIndex() int64 {
	return c.lastCaptureIndex
}

// process processes cmd
func (c *Command) process(opt *CommonOptions, cmd string) (err error) {
	// 如果语音切片和图片都输出的话创建 SliceOutputDir 和 CaptureOutputDir
	// 在开始前先把目录清除干净
	if opt.SliceAndCapture {
		if opt.HasSpeech {
			os.RemoveAll(opt.SliceOutputDir)
			err = os.MkdirAll(opt.SliceOutputDir, os.ModePerm)
			if err != nil {
				c.markError(err)
				return fmt.Errorf("error creating ffmpeg slice output directory: %w", err)
			}
		}
		if opt.HasVideo {
			os.RemoveAll(opt.CaptureOutputDir)
			err = os.MkdirAll(opt.CaptureOutputDir, os.ModePerm)
			if err != nil {
				c.markError(err)
				return fmt.Errorf("error creating ffmpeg capture output directory: %w", err)
			}
		}
	} else {
		os.RemoveAll(opt.OutputDir)
		err = os.MkdirAll(opt.OutputDir, os.ModePerm)
		if err != nil {
			c.markError(err)
			return fmt.Errorf("error creating ffmpeg output directory: %w", err)
		}
	}
	if opt.DecodeSEI {
		os.RemoveAll(opt.SEIOutputDir)
		err = os.MkdirAll(opt.SEIOutputDir, os.ModePerm)
		if err != nil {
			c.markError(err)
			return fmt.Errorf("error creating ffmpeg SEI output directory: %w", err)
		}
	}
	go func() {
		c.startWatching()
	}()
	c.cmd, err = c.exec(cmd)
	if err != nil {
		c.markError(err)
		return err
	}

	pid := 0
	if c.cmd.Process != nil {
		pid = c.cmd.Process.Pid
	}
	log.Info().Str("cmd", c.cmd.String()).Int("pid", pid).Str("mediaId", opt.MediaId).
		Msg("started ffmpeg process")

	return nil
}

// getVideoDuration returns video duration
func (c *Command) getVideoDuration(opt *ProbeOptions) (dur float64, err error) {
	st, err := c.ProbeStreams(opt)
	if err != nil {
		return 0, err
	}
	idx, ok := st.HasVideoStream()
	if !ok {
		return 0, fmt.Errorf("resource has no video stream")
	}

	dur, err = st.Streams[idx].GetDuration()
	if err != nil {
		return 0, fmt.Errorf("can not get video duration: %v", err)
	}
	return
}

// exec creates a new process through exec.Command, wait for it to finish in a goroutine
func (c *Command) exec(params string) (*exec.Cmd, error) {
	ow := new(bytes.Buffer)
	ew := new(bytes.Buffer)
	cmd := exec.Command("/bin/bash", "-c", params)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = ow
	cmd.Stderr = ew

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c.stats.Start = time.Now()
	c.started = true

	go func() {
		err := cmd.Wait()
		c.ffmpegExit = true
		c.stats.End = time.Now()
		log.Trace().Err(err).Msg("ffmpeg process finished")

		err = convertError(err, ew.String())
		if err != nil && err != ErrStreamClosed {
			c.markError(err)
			log.Err(err).Msg("ffmpeg process error")
			return
		}
		c.markFinished()

		// there may still be warning messages
		if ew.Len() != 0 {
			log.Warn().Msg(ew.String())
		}
		if ow.Len() != 0 {
			log.Info().Msg(ow.String())
		}
	}()

	return cmd, nil
}

// execwait creates a new process through exec.Command, blocks until it finishes
func (c *Command) execwait(params string, w io.Writer) (err error) {
	ew := new(bytes.Buffer)
	cmd := exec.Command("/bin/bash", "-c", params)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = w
	cmd.Stderr = ew
	if err = cmd.Start(); err != nil {
		return
	}
	if err = cmd.Wait(); err != nil {
		return convertError(err, ew.String())
	}

	// there may still be warning messages
	if ew.Len() != 0 {
		log.Warn().Msg(ew.String())
	}
	return nil
}

// kill kills underlying process
func (c *Command) kill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process not found or already stopped")
	}

	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		return err
	}

	return nil
}

// markError records error
func (c *Command) markError(err error) {
	if c.err == nil {
		c.err = err
	}
	return
}

// startWatching creates output directory and start watching file changes
func (c *Command) startWatching() {
	// Iterate over output directory until command closed
	for !c.ffmpegExit && c.err == nil {
		// 如果切片和截帧的话，那么返回这两个目录中切片生成的文件
		if c.opt.SliceAndCapture {
			if c.opt.HasVideo {
				c.handleNewFile(c.opt.CaptureOutputDir)
			}
			if c.opt.HasSpeech {
				c.handleNewFile(c.opt.SliceOutputDir)
			}
		} else {
			c.handleNewFile(c.opt.OutputDir)
		}
	}
}

func (c *Command) handleNewFile(dir string) bool {
	var (
		err   error
		files []os.DirEntry
		max   int64 = -1
		index int64 = -1
	)
	files, err = os.ReadDir(dir)
	if err != nil {
		c.markError(err)
		return true
	}
	for _, e := range files {
		index, err = utils.FilePath2Index(e.Name())
		if err != nil {
			c.markError(err)
			return true
		}
		if index != -1 && index > max {
			c.handleFileEvent(e.Name(), dir)
			max = index
		}
	}
	return false
}

// handleFileEvent handles file event
func (c *Command) handleFileEvent(name string, dir string) {
	// 1. Parse current output file index
	idx, err := utils.FilePath2Index(name)
	suffix := utils.FilePath2Suffix(name)
	if err != nil {
		log.Err(err).Str("file", name).Msg("error converting file path to index")
		c.markError(err)
		return
	}
	if idx <= 0 {
		// not log
		//log.Info().Str("file", name).Msg("skipping file event (index < 0)")
		return
	}

	// 2. Read previous output file content
	var (
		byt     []byte   // previous file content
		prev    string   // previous file name
		sei     []byte   // previous SEI file content
		prevSEI string   // previous SEI file name
		seiInfo []string // decoded SEI info
	)
	byt, prev, err = readPreviousFile(dir, suffix, idx)
	if err != nil {
		c.markError(err)
		return
	}
	if len(byt) <= 0 {
		// do not print log
		//log.Info().Str("file", prev).Msg("skipping file event (empty content)")
		return
	}

	// 3. When SEI is required, try to read previous SEI file
	if c.opt.DecodeSEI {
		for {
			sei, prevSEI, err = readPreviousFile(c.opt.SEIOutputDir, c.opt.SEIFragmentSuffix, idx)
			if len(sei) > 0 {
				seiInfo, err = c.decodeSEIInfo(sei)
				if err != nil {
					log.Error().Str("file", prevSEI).Err(err).Msg("error decoding SEI info")
					c.markError(err)
					return
				}
				break
			}
			time.Sleep(5 * time.Nanosecond)
		}
	}

	// 4. Create and modify the output, enqueue output file
	o := &Output{
		Content: byt,
		Index:   idx - 1,
		Last:    false, // 在此阶段一定没有结束
		SEIInfo: seiInfo,
		Suffix:  suffix,
	}
	c.mod(o) /* modify the output, populate any necessary info */
	if !c.closed {
		c.enqueue(c.opt, o)
	}

	// 5. Clean up processed files
	c.remove(prev)
	c.remove(prevSEI)
}

// enqueue pushes output file into queue
func (c *Command) enqueue(opt *CommonOptions, o *Output) {
	c.q.Enqueue(o)
	c.lastQueued += 1
	c.stats.Output += 1
	c.stats.Bytes += int64(len(o.Content))
	log.Trace().Int64("index", o.Index).Int("bytes", len(o.Content)).
		Float64("second", o.Second).Int64("position", o.Position).
		Str("mediaId", opt.MediaId).Msg("enqueued ffmpeg output file")
}

// remove deletes specified file when PreserveOutput is disabled
func (c *Command) remove(filename string) {
	if !c.opt.PreserveOutput {
		_ = os.Remove(filename)
	}
}

func (c *Command) removeDir(dir string) {
	if !c.opt.PreserveOutput {
		_ = os.RemoveAll(dir)
	}
}

// markFinished traverses output directory to update c.lastIndex with the lasted output file index when FFmpeg ends
func (c *Command) markFinished() {
	// Read output directory to find out the latest output file
	var (
		files []os.DirEntry
		err   error
	)
	if c.opt.SliceAndCapture {
		if c.opt.HasVideo {
			defer c.removeDir(c.opt.CaptureOutputDir)
			files, err = os.ReadDir(c.opt.CaptureOutputDir)
			if err != nil {
				log.Err(err).Bool("isSliceAndCapture", c.opt.SliceAndCapture).Str("dir", c.opt.SliceOutputDir).
					Msg("failed to read output directory")
				c.lastCaptureIndex = c.lastQueued
			}
			err = c.enqueueRemainingFiles(files, OutputTypeImage, c.opt.CaptureOutputDir)
			if err != nil {
				c.markError(err)
			}
		} else {
			c.finishCapture = true
		}
		if c.opt.HasSpeech {
			defer c.removeDir(c.opt.SliceOutputDir)
			files, err = os.ReadDir(c.opt.SliceOutputDir)
			if err != nil {
				log.Err(err).Bool("isSliceAndCapture", c.opt.SliceAndCapture).Str("dir", c.opt.SliceOutputDir).
					Msg("failed to read output directory")
				c.lastSliceIndex = c.lastQueued
			}
			err = c.enqueueRemainingFiles(files, OutputTypeAudioSegment, c.opt.SliceOutputDir)
			if err != nil {
				c.markError(err)
			}
		} else {
			c.finishedSlice = true
		}
	} else {
		defer c.removeDir(c.opt.OutputDir)
		files, err = os.ReadDir(c.opt.OutputDir)
		if err != nil {
			log.Err(err).Bool("isSliceAndCapture", c.opt.SliceAndCapture).Str("dir", c.opt.SliceOutputDir).
				Msg("failed to read output directory")
			c.lastCaptureIndex = c.lastQueued
		}
		err = c.enqueueRemainingFiles(files, OutputTypeAudioSegment, c.opt.OutputDir)
		if err != nil {
			c.markError(err)
		}
	}
	c.finished = true
	c.finishedAt = time.Now()
	log.Info().Int64("lastSliceIndex", c.lastSliceIndex).Int64("lastCapturedIndex", c.lastCaptureIndex).
		Int64("lastQueued", c.lastQueued).Msgf("mark finished")
}

func (c *Command) enqueueRemainingFiles(files []os.DirEntry, typ int, dir string) (err error) {
	var (
		last         bool
		lastCaptured bool
		lastSlice    bool
		maxIndex     int64
	)
	for _, f := range files {
		idx, _ := utils.FilePath2Index(f.Name())
		if idx > maxIndex {
			maxIndex = idx
		}
	}
	for _, f := range files {
		idx, _ := utils.FilePath2Index(f.Name())
		suffix := utils.FilePath2Suffix(f.Name())
		byt, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", dir, f.Name()))
		if err != nil {
			log.Error().Str("file", f.Name()).Msg("read fragment error")
			return err
		}
		if len(byt) <= 0 {
			log.Info().Str("file", f.Name()).Msg("skipping file event (empty content)")
			continue
		}
		if c.opt.SliceAndCapture {
			switch typ {
			case OutputTypeAudioSegment:
				last = c.finishCapture && idx == maxIndex
				lastSlice = idx == maxIndex
			case OutputTypeImage:
				last = c.finishedSlice && idx == maxIndex
				lastCaptured = idx == maxIndex
			}
		} else {
			last = idx == maxIndex
		}
		o := &Output{
			Content:      byt,
			Index:        idx,
			Last:         last,
			LastCaptured: lastCaptured,
			LastSliced:   lastSlice,
			Suffix:       suffix,
		}
		c.mod(o)
		if !c.closed {
			c.enqueue(c.opt, o)
		}
		c.remove(fmt.Sprintf("%s/%s", dir, f.Name()))
	}
	if c.opt.SliceAndCapture {
		switch typ {
		case OutputTypeAudioSegment:
			c.lastSliceIndex = maxIndex
			c.finishedSlice = true
		case OutputTypeImage:
			c.lastCaptureIndex = maxIndex
			c.finishCapture = true
		}
	} else {
		c.lastIndex = maxIndex
	}
	return nil
}

// decodeSEIInfo decodes SEI info from raw content
func (c *Command) decodeSEIInfo(byt []byte) ([]string, error) {
	all := SEIRegex.FindAll(byt, -1)
	result := make([]string, 0)
	for _, match := range all {
		mp := make(map[string]interface{})
		err := json.Unmarshal(match, &mp)
		if err != nil {
			continue
		}
		result = append(result, string(match))
	}
	return result, nil
}

// completeRead tries to read a just created file that may still being writen
func completeRead(fn string) ([]byte, error) {
	fi, err := os.Stat(fn)
	if err != nil {
		return nil, fmt.Errorf("stat file info error: %w", err)
	}
	l := fi.Size()

	time.Sleep(time.Millisecond * 100)

	// Read multiple times until file size is not zero and does not continue growing, break
	i := 1
	for i = 1; i <= 4; i++ {
		fi, err = os.Stat(fn)
		if err != nil {
			return nil, fmt.Errorf("stat file info error: %w", err)
		}
		l2 := fi.Size()
		if l2 > 0 && l2 == l {
			break
		}
		l = l2
		time.Sleep(time.Millisecond * 100)
	}

	if l == 0 {
		log.Trace().Int("times", i).Str("name", fn).Int64("bytes", l).Msg("complete read file")
	} else {
		// log.Trace().Int("times", i).Int64("bytes", l).Msg("complete read file")
	}
	return ioutil.ReadFile(fn)
}

// readPreviousFile tries to read previous file (having index equals idx-1), returns content and filename
func readPreviousFile(dir string, suffix string, idx int64) ([]byte, string, error) {
	fn := fmt.Sprintf(fmt.Sprintf("%s/%012d.%s", dir, idx-1, suffix))
	_, err := os.Stat(fn)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("stat file info error: %w", err)
	}
	byt, err := ioutil.ReadFile(fn)
	return byt, fn, err
}

// ParseCaptureCommand parses capture command string
func ParseCaptureCommand(opt *CaptureOptions) string {
	cmd := make([]string, 0)

	if com := ParseCommonOptions(&opt.CommonOptions, "ffmpeg", true); com != "" {
		cmd = append(cmd, com)
	}

	if capturedCmd := ParseCaptureOptions(opt); capturedCmd != "" {
		cmd = append(cmd, capturedCmd)
	}

	return strings.Join(cmd, space)
}

// ParseSliceCommand parses slice command string
func ParseSliceCommand(opt *SliceOptions) string {
	cmd := make([]string, 0)

	if com := ParseCommonOptions(&opt.CommonOptions, "ffmpeg", true); com != "" {
		cmd = append(cmd, com)
	}

	if sliceCmd := ParseSliceOptions(opt); sliceCmd != "" {
		cmd = append(cmd, sliceCmd)
	}

	return strings.Join(cmd, space)
}

// ParseSliceAndCaptureCommand parses slice and capture command string
func ParseSliceAndCaptureCommand(opt *SliceAndCaptureOptions) string {

	cmd := make([]string, 0)

	if com := ParseCommonOptions(&opt.CommonOptions, "ffmpeg", true); com != "" {
		cmd = append(cmd, com)
	}

	opt.Suffixes = append(opt.Suffixes, opt.CaptureOptions.Suffix)
	if opt.HasVideo {
		if capturedCmd := ParseCaptureOptions(opt.CaptureOptions); capturedCmd != "" {
			cmd = append(cmd, capturedCmd)
		}
	}

	opt.Suffixes = append(opt.Suffixes, opt.SliceOptions.Suffix)
	if opt.HasSpeech {
		if sliceCmd := ParseSliceOptions(opt.SliceOptions); sliceCmd != "" {
			cmd = append(cmd, sliceCmd)
		}
	}

	return strings.Join(cmd, space)
}

// ParseProbeCommand parses probe command string
func ParseProbeCommand(opt *ProbeOptions) string {
	cmd := make([]string, 0)

	commonOpt := &CommonOptions{
		Uri:           opt.Uri,
		IsStream:      opt.IsStream,
		IsFile:        opt.IsFile,
		Proxy:         opt.Proxy,
		LogLevel:      opt.LogLevel,
		DockerCommand: opt.DockerCommand,
	}

	if com := ParseCommonOptions(commonOpt, "ffprobe", true); com != "" {
		cmd = append(cmd, com)
	}

	cmd = append(cmd, probe)

	return strings.Join(cmd, space)
}

// ParseSliceOptions parses slice options string
func ParseSliceOptions(opt *SliceOptions) string {
	cmd := make([]string, 0)

	if opt.DisableVideo && !opt.DecodeSEI {
		cmd = append(cmd, "-vn")
	}
	if opt.Coding != "" {
		cmd = append(cmd, "-c:a", opt.Coding)
	}
	if opt.SamplingFrequency != 0 {
		cmd = append(cmd, "-ar", fmt.Sprintf("%v", opt.SamplingFrequency))
	}
	if opt.Channels != 0 {
		cmd = append(cmd, "-ac", fmt.Sprintf("%v", opt.Channels))
	}
	if opt.Format != "" {
		cmd = append(cmd, "-f", opt.Format)
	}
	if opt.FragmentDuration != 0 {
		cmd = append(cmd, "-segment_time", fmt.Sprintf("%v", opt.FragmentDuration))
	}
	cmd = append(cmd, fmt.Sprintf("%s/%%012d.%s", opt.OutputDir, opt.Suffix))
	if opt.DecodeSEI {
		cmd = append(cmd, "-c copy -f segment -segment_time", fmt.Sprintf("%v", opt.FragmentDuration))
		cmd = append(cmd, fmt.Sprintf("%s/%%012d.%s", opt.SEIOutputDir, opt.SEIFragmentSuffix))
	}
	cmd = append(cmd, "-y")

	return strings.Join(cmd, space)
}

// ParseCaptureOptions parses capture options string
func ParseCaptureOptions(opt *CaptureOptions) string {
	cmd := make([]string, 0)

	if opt.MaxFrames > 0 /* limit maximum number of captured images */ {
		cmd = append(cmd,
			"-t", fmt.Sprintf("%vs", utils.GetMaxFrameLimit(opt.MaxFrames, opt.Rate)))
	}
	vf := fmt.Sprintf(filterInterval, 1/opt.Rate) /* capture according to specified interval */
	if opt.Mode == CaptureModeByFrame {
		vf = fmt.Sprintf(filterEveryNFrm, opt.Frame) /* capture according to specified frame */
	}
	if opt.Debug {
		vf = filterDebug + "," + vf
	}
	cmd = append(
		cmd,
		"-vf", fmt.Sprintf("'%v'", vf),
		"-r", fmt.Sprintf("%v", opt.Rate),
		"-f", "image2", // output format
		"-qscale:v", "1", // image quality options
		"-qmin", "1", // image quality options
	)
	if opt.Size != "" /* specify captured image size */ {
		cmd = append(cmd, "-s", opt.Size)
	}
	cmd = append(cmd, fmt.Sprintf("%s/%%012d.%s", opt.OutputDir, opt.Suffix), "-y")
	return strings.Join(cmd, space)
}

// ParseCommonOptions returns a command options string
func ParseCommonOptions(opt *CommonOptions, command string, withUri bool) string {

	cmd := make([]string, 0)

	cmd = append(cmd, ParseCommandWithoutArguments(opt, command)...)

	cmd = append(cmd, common, "-loglevel", opt.GetLogLevel())

	if opt.IsStream /* Only append tw_timeout options for streams */ {
		cmd = append(cmd, fmt.Sprintf(rwTimeout, opt.GetIOTimeout()*1000000))
	}

	if !(opt.IsFile || opt.IsStream) /* Only append reconnect & proxy options for video/audio url */ {
		cmd = append(cmd, reconnect)
		if opt.Proxy != "" {
			cmd = append(cmd, "-http_proxy", opt.HttpProxy())
		}
	}

	if withUri {
		cmd = append(cmd, "-i", fmt.Sprintf("'%v'", opt.Uri))
	}

	return strings.Join(cmd, space)
}

// ParseCommandWithoutArguments returns a command string without arguments
func ParseCommandWithoutArguments(opt *CommonOptions, command string) []string {

	cmd := make([]string, 0)
	if opt.DockerCommand != "" {
		cmd = append(cmd, opt.DockerCommand)
	} else {
		cmd = append(cmd, command)
	}
	return cmd
}
