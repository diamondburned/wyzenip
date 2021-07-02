package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/catnipgtk"
	"github.com/diamondburned/wyzenip/internal/catnipdraw"
	"github.com/noriah/catnip/input"
	"github.com/pkg/errors"

	// Input backends.
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
)

const bars = 2

func main() {
	username := os.Getenv("admin_username")
	password := os.Getenv("admin_password")
	if username == "" || password == "" {
		log.Fatalln("missing $admin_{username,password}")
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	drawer := catnipdraw.NewDrawer(cfg, 10)

	drawDone := make(chan error)
	go func() {
		drawDone <- drawer.Start(ctx)
		cancel()
	}()

	bulb := Bulb{
		username: username,
		password: password,
		endpoint: "http://192.168.1.7",
	}

	lastTime := time.Now()

mainLoop:
	for {
		select {
		case err = <-drawDone:
			break mainLoop
		default:
		}

		var intensity int

		drawer.View(func(bars [][]input.Sample) {
			perc := math.Min(bars[0][2], 15) / 15 * 100
			intensity = int(math.Round(perc))
		})

		if err := bulb.ChangeIntensity(ctx, intensity); err != nil {
			log.Fatalln("failed to change intensity:", err)
		}

		now := time.Now()
		fmt.Printf("\rLatency: %dms", now.Sub(lastTime).Milliseconds())
		lastTime = now
	}

	if err != nil {
		log.Fatalln("failed to start:", err)
	}
}

type Bulb struct {
	username string
	password string
	endpoint string
}

func (b Bulb) ChangeIntensity(ctx context.Context, intensity int) error {
	if intensity < 0 || intensity > 100 {
		return fmt.Errorf("intensity %d out of range", intensity)
	}

	bulbValues := url.Values{
		"m":  {"1"},
		"d0": {strconv.Itoa(intensity)},
	}

	r, err := http.NewRequestWithContext(ctx, "GET", b.endpoint+"/?"+bulbValues.Encode(), nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	r.SetBasicAuth(b.username, b.password)

	response, err := http.DefaultClient.Do(r)
	if err != nil {
		return errors.Wrap(err, "failed to do request")
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d", response.StatusCode)
	}

	return nil
}
