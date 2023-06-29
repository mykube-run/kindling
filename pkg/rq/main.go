package rq

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"time"
)

var (
	// DefaultTimeout is the default timeout per request
	DefaultTimeout = time.Second * 5
	// DefaultRetryCount is the default retry count
	DefaultRetryCount = 2
	// GlobalTransport is the global transport
	// NOTE: When using proxy, CREATE A NEW TRANSPORT to avoid affecting requests that do not need proxies.
	GlobalTransport = &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
	}
	// DefaultClient is the default request client
	DefaultClient = resty.New().SetLogger(NewRestyLogger()).
			SetTransport(GlobalTransport).
			SetTimeout(DefaultTimeout).
			SetRetryCount(DefaultRetryCount)
)

// NewClient creates a new resty client using the global transport (with timeout default to 2s and retry at most 2 times)
func NewClient() *resty.Client {
	return NewClientWithTransport(GlobalTransport)
}

// NewClientWithTransport creates a new resty client from given transport
func NewClientWithTransport(tran http.RoundTripper) *resty.Client {
	c := resty.New().SetLogger(NewRestyLogger()).
		SetTransport(tran).
		SetTimeout(DefaultTimeout).
		SetRetryCount(DefaultRetryCount)
	return c
}

// IsTimeout checks whether err is timeout
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}

	e, ok := err.(*url.Error)
	return ok && e.Timeout()
}

// Trace returns trace information about the request
func Trace(res *resty.Response) resty.TraceInfo {
	if res != nil && res.Request != nil {
		return res.Request.TraceInfo()
	}
	return resty.TraceInfo{}
}

// RestyLogger implements resty logger
type RestyLogger struct {
}

func NewRestyLogger() *RestyLogger {
	return &RestyLogger{}
}

func (r RestyLogger) Errorf(format string, v ...interface{}) {
	log.Err(fmt.Errorf(format, v...)).Msgf("request error")
}

func (r RestyLogger) Warnf(format string, v ...interface{}) {
	log.Warn().Msgf(format, v...)
}

func (r RestyLogger) Debugf(format string, v ...interface{}) {
	log.Debug().Msgf(format, v...)
}
