# tor-prebuilt

embed tor daemon within your programs.

It builds an embedded copy of the tor daemon from the TBB archives available from the official website.

The api is compatible with [github.com/cretz/bine](https://github.com/cretz/bine)

# Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/clementauger/tor-prebuilt/embedded"
)

func main() {
	// Start tor with default config (can set start conf's DebugWriter to os.Stdout for debug logs)
	fmt.Println("Starting and registering onion service, please wait a couple of minutes...")
	t, err := tor.Start(nil, &tor.StartConf{ProcessCreator: embedded.NewCreator(), TorrcFile:"torrc-defaults"})
	if err != nil {
		log.Panicf("Unable to start Tor: %v", err)
	}
	defer t.Close()
	// Wait at most a few minutes to publish the service
	listenCtx, listenCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer listenCancel()
	// Create a v3 onion service to listen on any port but show as 80
	onion, err := t.Listen(listenCtx, &tor.ListenConf{Version3: true, RemotePorts: []int{80}})
	if err != nil {
		log.Panicf("Unable to create onion service: %v", err)
	}
	defer onion.Close()
	fmt.Printf("Open Tor browser and navigate to http://%v.onion\n", onion.ID)
	fmt.Println("Press enter to exit")
	// Serve the current folder from HTTP
	errCh := make(chan error, 1)
	go func() { errCh <- http.Serve(onion, http.FileServer(http.Dir("."))) }()
	// End when enter is pressed
	go func() {
		fmt.Scanln()
		errCh <- nil
	}()
	if err = <-errCh; err != nil {
		log.Panicf("Failed serving: %v", err)
	}
}
```

# What for ?

Benefits of the tor network to host your application at home and make it accessible to anyone in a glance.

# Why ?

Because the other alternative, i am aware of, to include tor connectivity to a go program, requires
a more complex and much harder to maintain build chain.

The method implemented in this package might appear dirtier,
yet, it is more effective.

# Build

`make all`

# Notes

I try to keep it this repository regularly up to date.
But, I cannot test windows and macos version for each releases.
If I had the resources i would setup a system to watch for tor website updates and
keep this repository automatically updated with little human surveillance for breakages.

Ideally, someone writes a module to keep the binary dependencies up to date upon
starting of an application.

The internal versionning of the tor engine builds follows the TBB versionning.
