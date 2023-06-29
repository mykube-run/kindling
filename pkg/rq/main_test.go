package rq

import (
	"testing"
	"time"
)

func TestTimeoutError(t *testing.T) {
	url := "https://httpbin.org/delay/3" // delay 3 second
	_, err := NewClient().SetTimeout(time.Second).R().Get(url)
	if !IsTimeout(err) {
		t.Fatalf("expected err to be timeout error, but got: %v", err)
	}
	_, err = NewClient().SetTimeout(time.Second * 5).R().Get(url)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
