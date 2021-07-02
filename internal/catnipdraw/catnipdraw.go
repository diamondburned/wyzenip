package catnipdraw

import (
	"context"
	"sync"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/catnipgtk"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/fft"
	"github.com/noriah/catnip/input"
	"github.com/pkg/errors"
)

// chCount is 1 because we're only checking 1 channel.
const chCount = 2

// Drawer is a drawer instance.
type Drawer struct {
	bars   int
	config catnip.Config

	backend input.Backend
	device  input.Device

	fftPlans []*fft.Plan
	fftBufs  [][]complex128
	spectrum dsp.Spectrum

	shared struct {
		sync.Mutex
		// Input buffers.
		readBuf  [][]input.Sample
		writeBuf [][]input.Sample
		// Output bars.
		barBufs [][]input.Sample
		barView [][]input.Sample // subslice of barBufs
	}
}

// NewDrawer creates a new drawer.
func NewDrawer(cfg *catnipgtk.Config, bars int) *Drawer {
	return &Drawer{
		bars:    bars,
		config:  cfg.Transform(),
		backend: cfg.Input.InputBackend(),
		device:  cfg.Input.InputDevice(),
	}
}

// Start starts the area. This function blocks permanently until the audio loop
// is dead, so it should be called inside a goroutine. This function should not
// be called more than once, else it will panic.
//
// The loop will automatically close when the DrawingArea is destroyed.
func (d *Drawer) Start(ctx context.Context) (err error) {
	session, err := d.init()
	if err != nil {
		return err
	}

	// Write to writeBuf, and we can copy from write to read (see Process).
	if err := session.Start(ctx, d.shared.writeBuf, d); err != nil {
		return errors.Wrap(err, "failed to start input session")
	}

	return nil
}

func (d *Drawer) init() (input.Session, error) {
	d.shared.Lock()
	defer d.shared.Unlock()

	if d.shared.barBufs != nil {
		// Panic is reasonable, as calling Start() multiple times (in multiple
		// goroutines) may cause undefined behaviors.
		panic("BUG: catnip.Area is already started.")
	}

	d.spectrum = dsp.Spectrum{
		SampleRate: d.config.SampleRate,
		SampleSize: d.config.SampleSize,
		Bins:       make(dsp.BinBuf, d.config.SampleSize),
	}
	d.spectrum.Recalculate(d.bars)
	d.spectrum.SetSmoothing(d.config.SmoothFactor / 100)
	d.spectrum.SetType(dsp.TypeLog)

	sessionConfig := input.SessionConfig{
		Device:     d.device,
		FrameSize:  chCount,
		SampleSize: d.config.SampleSize,
		SampleRate: d.config.SampleRate,
	}

	session, err := d.backend.Start(sessionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start the input backend")
	}

	// Invalidate the device.
	// device = nil
	sessionConfig.Device = nil

	// Allocate buffers.
	d.reallocBarBufs()
	d.reallocFFTBufs()
	d.shared.readBuf = input.MakeBuffers(sessionConfig)
	d.shared.writeBuf = input.MakeBuffers(sessionConfig)

	// Initialize the FFT plans.
	d.fftPlans = make([]*fft.Plan, chCount)
	for idx := range d.fftPlans {
		plan := fft.Plan{
			Input:  d.shared.readBuf[idx],
			Output: d.fftBufs[idx],
		}
		plan.Init()
		d.fftPlans[idx] = &plan
	}

	return session, nil
}

// View views into the internal bar buffer. The outer slice is the slice of
// channels, and the inner slice is the slice of bars.
func (d *Drawer) View(f func(bars [][]input.Sample)) {
	d.shared.Lock()

	if d.shared.barBufs != nil {
		d.updateBars()
		f(d.shared.barView)
	}

	d.shared.Unlock()
}

// updateBars updates d.barBufs and such and returns true if a redraw is needed.
func (d *Drawer) updateBars() {
	for idx, buf := range d.shared.barBufs {
		d.config.WindowFn(d.shared.readBuf[idx])
		d.fftPlans[idx].Execute() // process into buf

		d.spectrum.Process(buf, d.fftBufs[idx])
	}
}

func (d *Drawer) Process() {
	d.shared.Lock()
	defer d.shared.Unlock()

	// Copy the audio over.
	input.CopyBuffers(d.shared.readBuf, d.shared.writeBuf)
}

func (d *Drawer) reallocBarBufs() {
	// Allocate a large slice with one large backing array.
	fullBuf := make([]float64, chCount*d.config.SampleSize)

	// Allocate smaller slice views.
	barBufs := make([][]float64, chCount)
	barView := make([][]float64, chCount)

	for idx := range barBufs {
		start := idx * d.config.SampleSize
		end := (idx + 1) * d.config.SampleSize

		barBufs[idx] = fullBuf[start:end]
		barView[idx] = barBufs[idx][:d.bars]
	}

	d.shared.barBufs = barBufs
	d.shared.barView = barView
}

func (d *Drawer) reallocFFTBufs() {
	eachLen := d.config.SampleSize/2 + 1

	// Allocate a large slice with one large backing array.
	fullBuf := make([]complex128, chCount*eachLen)

	// Allocate smaller slice views.
	fftBufs := make([][]complex128, chCount)

	for idx := range fftBufs {
		start := idx * eachLen
		end := (idx + 1) * eachLen

		fftBufs[idx] = fullBuf[start:end]
	}

	d.fftBufs = fftBufs
}
