package main

import (
"fmt"
"github.com/libp2p/go-libp2p"
"github.com/libp2p/go-libp2p/p2p/host/resource-manager"
"github.com/libp2p/go-libp2p/x/rate"
"log"
)

func main() {
    limiter := rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits)
    rm, err := rcmgr.NewResourceManager(
limiter, 
rcmgr.WithConnRateLimiters(&rate.Limiter{
			GlobalLimit: rate.Limit{},
		}),
	)
    if err != nil {
        log.Fatal(err)
    }

	_, err = libp2p.New(
libp2p.ResourceManager(rm),
)
	if err != nil {
		log.Fatal(err)
	}
    fmt.Println("Success")
}
