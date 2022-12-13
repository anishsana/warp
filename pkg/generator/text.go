/*
 * Warp (C) 2019-2020 MinIO, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package generator

import (
	cRand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync/atomic"
)

func WithTextData() TextOpts {
	return textOptsDefaults()
}

// Apply Text data options.
func (o TextOpts) Apply() Option {
	return func(opts *Options) error {
		if err := o.validate(); err != nil {
			return err
		}
		opts.text = o
		opts.src = newText
		return nil
	}
}

func (o TextOpts) validate() error {
	if o.size <= 0 {
		return errors.New("text: size <= 0")
	}
	return nil
}

// RngSeed will which to a fixed RNG seed to make usage predictable.
func (o TextOpts) RngSeed(s int64) TextOpts {
	o.seed = &s
	return o
}

// Size will set a block size.
// Data of this size will be repeated until output size has been reached.
func (o TextOpts) Size(s int) TextOpts {
	o.size = s
	return o
}

// TextOpts are the options for the text data source.
type TextOpts struct {
	seed *int64
	size int
}

func textOptsDefaults() TextOpts {
	return TextOpts{
		seed: nil,
		// Use 128KB as base.
		size: 128 << 10,
	}
}

type textSrc struct {
	counter uint64
	o       Options
	buf     *circularBuffer
	rng     *rand.Rand
	obj     Object
}

func newText(o Options) (Source, error) {
	txtSrc := rand.NewSource(int64(rand.Uint64()))
	if o.text.seed != nil {
		txtSrc = rand.NewSource(*o.text.seed)
	}
	rng := rand.New(txtSrc)

	size := o.text.size
	if int64(size) > o.totalSize {
		size = int(o.totalSize)
	}
	if size <= 0 {
		return nil, fmt.Errorf("size must be >= 0, got %d", size)
	}

	// Seed with random data.
	data := make([]byte, size)
	_, err := io.ReadFull(rng, data)
	if err != nil {
		return nil, err
	}

	t := textSrc{
		o:   o,
		rng: rng,
		buf: newCircularBuffer(data, int64(size)),
		obj: Object{
			Reader:      nil,
			Name:        "",
			ContentType: "text/plain",
			Size:        0,
		},
	}
	t.obj.setPrefix(o)
	return &t, nil
}

func (t *textSrc) Object() *Object {
	atomic.AddUint64(&t.counter, 1)

	t.obj.Size = t.o.getSize(t.rng)

	// build data until the desired size.
	builder := make([]byte, 0)
	for int64(len(builder)) < t.obj.Size {
		reqSize := t.obj.Size - int64(len(builder))
		builder = append(builder, genData(reqSize, t.o.compRatio, t.o.compWindow)...)
	}

	t.buf.data = builder

	var nBuf [16]byte
	randASCIIBytes(nBuf[:], t.rng)
	t.obj.setName(fmt.Sprintf("%d.%s.txt", atomic.LoadUint64(&t.counter), string(nBuf[:])))

	// Reset scrambler
	t.obj.Reader = t.buf.Reset(t.obj.Size)
	return &t.obj
}

// generates compressible data with the provided compression ratio.
func genData(reqSize int64, compRatio int, compWindow int64) []byte {
	var uniqueStrLen int64
	var remStrLen int
	var repeatUniqueStrLen int64

	if compRatio > 0 && reqSize <= compWindow {
		uniqueStrLen = reqSize / int64(compRatio)
		remStrLen = int(reqSize % int64(compRatio))
		repeatUniqueStrLen = uniqueStrLen * int64(compRatio)

		// if required size is less than compression ratio, repeat single byte for required size.
		if uniqueStrLen == 0 {
			uniqueStrLen = 1
			remStrLen = 0
			repeatUniqueStrLen = reqSize
		}
	} else if compRatio > 0 {
		// restrict unique string to compression window
		uniqueStrLen = compWindow / int64(compRatio)
		remStrLen = int(compWindow % int64(compRatio))
		repeatUniqueStrLen = uniqueStrLen * int64(compRatio)
	} else {
		uniqueStrLen = reqSize
		remStrLen = 0
		repeatUniqueStrLen = uniqueStrLen
	}

	// build unique slice with random data; data will be incompressible
	uniqueStr := make([]byte, uniqueStrLen)
	_, err := cRand.Read(uniqueStr)
	if err != nil {
		fmt.Println("error:", err)
		return nil
	}

	builder := make([]byte, 0)

	// repeat full unique string
	for int64(len(builder)) != repeatUniqueStrLen {
		builder = append(builder, uniqueStr...)
	}

	// fill remaining length with part of unique string
	for i := 0; i < remStrLen; i++ {
		builder = append(builder, builder[i])
	}

	return builder
}

func (t *textSrc) String() string {
	if t.o.randSize {
		return fmt.Sprintf("Text data; random size up to %d bytes", t.o.totalSize)
	}
	return fmt.Sprintf("Text data; %d bytes total", t.buf.want)
}

func (t *textSrc) Prefix() string {
	return t.obj.Prefix
}
