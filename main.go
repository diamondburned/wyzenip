package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/catnipgtk"
	"github.com/diamondburned/wyzenip/internal/bulb"
	"github.com/diamondburned/wyzenip/internal/catnipdraw"
	"github.com/noriah/catnip/input"

	// Input backends.
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
)

const bars = 2

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing env var $%s", key)
	}
	return v
}

func main() {
	mqttURL := mustEnv("mqtt_url")
	if !strings.Contains(mqttURL, "://") {
		log.Fatalln("invalid mqtt URL missing scheme")
	}

	cfg, err := catnipgtk.ReadUserConfig()
	if err != nil {
		log.Fatalln("failed to read config:", err)
	}

	cfg.Visualizer.SampleRate = 16000
	cfg.Visualizer.SampleSize = 500
	cfg.Visualizer.WindowFn = catnipgtk.Rectangular
	cfg.Visualizer.Distribution = catnipgtk.DistributeEqual
	cfg.Visualizer.SmoothFactor = 0

	drawer := catnipdraw.NewDrawer(cfg, 10)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client, err := bulb.Connect(ctx, mqttURL)
	if err != nil {
		log.Fatalln("failed to init MQTT:", err)
	}
	defer client.Disconnect()

	drawDone := make(chan error)
	go func() {
		drawDone <- drawer.Start(ctx)
		cancel()
	}()

	updateTick := time.NewTicker(time.Second / 30)
	defer updateTick.Stop()

	dimmer := bulb.NewDimmer(client)
	dimmer.SetBlocking(true)

	lastTime := time.Now()

	var intensity int
	var changed bool

mainLoop:
	for {
		select {
		case err = <-drawDone:
			break mainLoop
		case <-updateTick.C:
			// ok
		}

		const (
			min = 85
			max = 100
			mod = 2
		)

		drawer.View(func(bars [][]input.Sample) {
			perc := math.Min(bars[0][2], 15) / 15 * (max - min)
			nint := roundMod(int(min+math.Round(perc)), mod)
			changed = nint != intensity
			intensity = nint
		})

		if changed {
			if err := dimmer.Dim(ctx, intensity); err != nil {
				log.Fatalln("failed to change intensity:", err)
			}
		}

		now := time.Now()
		fmt.Printf("\rLatency: %03dms, Intensity %d%%", now.Sub(lastTime).Milliseconds(), intensity)
		lastTime = now
	}

	if err != nil {
		log.Fatalln("failed to start:", err)
	}
}

func roundMod(i, mod int) int {
	return i - (i % mod)
}
