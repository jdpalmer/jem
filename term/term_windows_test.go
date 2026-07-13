//go:build windows

package term

import (
	"os"
	"testing"
	"time"
)

func TestTermWaitReadablePipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	SetTestInput(r)
	if termWaitReadable(termFd, 0) {
		t.Fatal("expected no data on empty pipe")
	}

	done := make(chan struct{})
	go func() {
		_, _ = w.Write([]byte("x"))
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for !termWaitReadable(termFd, 10*time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for pipe data")
		}
	}
	<-done
}

func TestTermWaitReadableInvalidFd(t *testing.T) {
	if termWaitReadable(-1, time.Millisecond) {
		t.Fatal("expected false for invalid fd")
	}
}
