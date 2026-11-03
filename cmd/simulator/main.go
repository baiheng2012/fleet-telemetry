package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/baiheng2012/fleet-telemetry/internal/contract"
	"github.com/baiheng2012/fleet-telemetry/internal/vehicle"
)

func main() {
	nLHD := flag.Int("lhd", 2, "number of LHD")
	nTruck := flag.Int("truck", 2, "number of Truck")
	nBolter := flag.Int("bolter", 1, "number of Bolter")
	interval := flag.Duration("interval", time.Second, "tick interval")
	seed := flag.Int64("seed", 42, "base seed")
	contractPath := flag.String("contract", "contracts/vehicle_semantics.yaml", "contract file")
	chaosOn := flag.Bool("chaos", true, "inject dirty data")
	flag.Parse()

	c, err := contract.Load(*contractPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	out := make(chan vehicle.Reading, 1024)

	// 唯一的写入协程（下周换成 NATS publisher）
	var writerDone sync.WaitGroup
	writerDone.Add(1)
	go func() {
		defer writerDone.Done()
		enc := json.NewEncoder(os.Stdout)
		for r := range out {
			if err := enc.Encode(r); err != nil {
				log.Println("encode:", err)
			}
		}
	}()

	var wg sync.WaitGroup
	idx := int64(0)
	start := func(t vehicle.Type, n int) {
		for i := 1; i <= n; i++ {
			idx++
			id := fmt.Sprintf("%s-%03d", t, i)
			v, err := vehicle.New(id, t, *seed+idx, c)
			if err != nil {
				log.Fatal(err)
			}
			var ch *vehicle.Chaos
			if *chaosOn {
				ch = vehicle.NewChaos(*seed+1000+idx, 0.01, 0.01, 0.02, 0.02)
			} else {
				ch = vehicle.NewChaos(0, 0, 0, 0, 0)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				run(ctx, v, ch, *interval, out)
			}()
		}
	}
	start(vehicle.LHD, *nLHD)
	start(vehicle.Truck, *nTruck)
	start(vehicle.Bolter, *nBolter)

	wg.Wait()
	close(out)
	writerDone.Wait()
	log.Println("simulator stopped")
}

func run(ctx context.Context, v *vehicle.Vehicle, ch *vehicle.Chaos, every time.Duration, out chan<- vehicle.Reading) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			for _, r := range ch.Apply(v.Tick(now.UTC())) {
				select {
				case out <- r:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
