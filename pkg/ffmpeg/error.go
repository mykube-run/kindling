package ffmpeg

import (
	"fmt"
	"strings"
)

var (
	ErrInvalidUrl            = fmt.Errorf("INVALID_URL")              // Not a valid url, 5xx or 400
	ErrUrlExpiredOrForbidden = fmt.Errorf("URL_EXPIRED_OR_FORBIDDEN") // Url expired or forbidden (403)
	ErrUrlNotFound           = fmt.Errorf("URL_NOT_FOUND")            // Url not found (404)
	ErrMediaServerError      = fmt.Errorf("MEDIA_SERVER_ERROR")       // Media server returned 5xx response
	ErrInvalidData           = fmt.Errorf("INVALID_DATA")             // Invalid data
	ErrResolveHostFailed     = fmt.Errorf("RESOLVE_HOST_FAILED")      // Failed to resolve hostname
	ErrConnectionTimeout     = fmt.Errorf("CONNECTION_TIMEOUT")       // Connection timeout
	ErrOOMKilled             = fmt.Errorf("OOM_KILLED")               // OOM killed
	ErrNoStream              = fmt.Errorf("NO_STREAM")                // No stream/does not contain any stream
	ErrStreamClosed          = fmt.Errorf("STREAM_CLOSED")            // Stream closed. This may be a normal result instead of a REAL ERROR
)

var errs = []knownError{
	{
		// File not exists
		ErrInvalidUrl, "No such file or directory",
	},
	{
		// Bad request
		ErrInvalidUrl, "Server returned 400 Bad Request",
	},
	{
		// Invalid protocol, e.g. xx.xx.xx.xx/sample.mp4 or C:\\Users\\sample.mp4: Protocol not found
		ErrInvalidUrl, "Protocol not found",
	},
	{
		// Invalid host, e.g. Failed to resolve hostname rongcloud...com: Name or service not known
		ErrInvalidUrl, "Name or service not known",
	},
	{
		// Url 403, may be expired or forbidden
		ErrUrlExpiredOrForbidden, "Server returned 403 Forbidden (access denied)",
	},
	{
		// Url 404
		ErrUrlNotFound, "Server returned 404 Not Found",
	},
	{
		ErrMediaServerError, "Server returned 5XX Server Error reply",
	},
	{
		// Invalid data
		ErrInvalidData, "Invalid data found when processing input",
	},
	{
		ErrResolveHostFailed, "Failed to resolve hostname",
	},
	{
		ErrConnectionTimeout, "Connection timed out",
	},
	{
		ErrConnectionTimeout, "Operation timed out",
	},
	{
		// Started too many ffmpeg processes, or exceeded POD memory limit
		ErrOOMKilled, "signal: killed",
	},
	{
		ErrNoStream, "not contain any stream",
	},
	{
		ErrStreamClosed, "failed to read RTMP packet",
	},
	{
		// Stream does not exist or finished
		ErrStreamClosed, "No such stream",
	},
	{
		// Stream server closed
		ErrStreamClosed, "Input/output error",
	},
}

type knownError struct {
	Error   error
	Message string
}

// convertError converts error from FFmpeg error message
func convertError(err error, msg string) error {
	if err == nil {
		return nil
	}
	for _, e := range errs {
		if strings.Contains(msg, e.Message) {
			return e.Error
		}
	}
	return fmt.Errorf("%v - %v", err, msg)
}
