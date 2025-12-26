package chip8

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

	"github.com/go-audio/audio"
	"github.com/go-audio/generator"
	"github.com/gordonklaus/portaudio"
)

const (
	bufferSize int     = 512
	note       float64 = 440.0
)

var (
	format = audio.FormatMono44100
)

type Beep struct {
	wg      sync.WaitGroup
	beeping atomic.Bool
}

func (b *Beep) Start(ctx context.Context) error {
	if b.beeping.Load() {
		return nil
	}
	b.beeping.Store(true)

	err := portaudio.Initialize()
	if err != nil {
		return err
	}

	buffer := &audio.FloatBuffer{
		Data:   make([]float64, bufferSize),
		Format: format,
	}

	osc := generator.NewOsc(generator.WaveSine, note, buffer.Format.SampleRate)
	osc.Amplitude = 1

	b.wg.Go(func() {
		defer func() {
			_ = portaudio.Terminate()
		}()

		out := make([]float32, bufferSize)

		stream, err := portaudio.OpenDefaultStream(0, 1, 44100, len(out), &out)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			_ = stream.Close()
		}()

		if err := stream.Start(); err != nil {
			log.Fatal(err)
		}
		defer func() {
			_ = stream.Stop()
		}()

		for b.beeping.Load() && ctx.Err() == nil {
			if err := osc.Fill(buffer); err != nil {
				log.Printf("error filling up the buffer")
			}

			f64Tof32(out, buffer.Data)

			if err := stream.Write(); err != nil {
				log.Printf("error writing to stream: %v\n", err)
			}
		}
	})

	return nil
}

func (b *Beep) Stop() {
	if !b.beeping.Load() {
		return
	}
	b.beeping.Store(false)
	b.wg.Wait()
}

func f64Tof32(dst []float32, src []float64) {
	for i := range src {
		dst[i] = float32(src[i])
	}
}
