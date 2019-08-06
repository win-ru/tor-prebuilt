package embedded_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/clementauger/tor-prebuilt/embedded"
)

func TestTor(t *testing.T) {
	args := []string{"-f", "torrc", "--defaults-torrc", "torrc-defaults"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	process, err := embedded.NewCreator().New(ctx, args...)
	if err != nil {
		t.Fatal(err)
	}
	err = process.Start()
	if err != nil {
		t.Fatal(err)
	}
	err = process.Wait()
	if err != nil && !strings.Contains(err.Error(), "killed") {
		t.Fatal(err)
	}
}
