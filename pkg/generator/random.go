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

func WithRandomData() RandomOpts {
	return randomOptsDefaults()
}

// Apply Random data options.
func (o RandomOpts) Apply() Option {
	return func(opts *Options) error {
		if err := o.validate(); err != nil {
			return err
		}
		opts.random = o
		opts.src = newRandom
		return nil
	}
}

func (o RandomOpts) validate() error {
	if o.size <= 0 {
		return errors.New("random: size <= 0")
	}
	return nil
}

// RngSeed will which to a fixed RNG seed to make usage predictable.
func (o RandomOpts) RngSeed(s int64) RandomOpts {
	o.seed = &s
	return o
}

// Size will set a block size.
// Data of this size will be repeated until output size has been reached.
func (o RandomOpts) Size(s int) RandomOpts {
	o.size = s
	return o
}

// RandomOpts are the options for the random data source.
type RandomOpts struct {
	seed *int64
	size int
}

func randomOptsDefaults() RandomOpts {
	return RandomOpts{
		seed: nil,
		// Use 128KB as base.
		size: 128 << 10,
	}
}

type randomSrc struct {
	counter uint64
	o       Options
	buf     *scrambler
	rng     *rand.Rand
	obj     Object
}

type plainSrc struct {
	o		Options
	rng		*rand.Rand
	obj     Object
	buf     *circularBuffer
	builder []byte
}

func newRandom(o Options) (Source, error) {
	c := plainSrc{
		o: o,
	}
	rndSrc := rand.NewSource(int64(rand.Uint64()))
	if o.random.seed != nil {
		rndSrc = rand.NewSource(*o.random.seed)
	}
	rng := rand.New(rndSrc)
	size := o.random.size
	// size := int(o.totalSize)
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
	c.builder = make([]byte, 0, o.totalSize)
	c.buf = newCircularBuffer(data, o.totalSize)
	c.rng = rng
	c.obj.ContentType = "text/plain"
	c.obj.Size = 0
	c.obj.setPrefix(o)

	return &c, nil
	// rndSrc := rand.NewSource(int64(rand.Uint64()))
	// if o.random.seed != nil {
	// 	rndSrc = rand.NewSource(*o.random.seed)
	// }
	// rng := rand.New(rndSrc)

	// size := o.random.size
	// if int64(size) > o.totalSize {
	// 	size = int(o.totalSize)
	// }
	// if size <= 0 {
	// 	return nil, fmt.Errorf("size must be >= 0, got %d", size)
	// }

	// // Seed with random data.
	// data := make([]byte, size)
	// _, err := io.ReadFull(rng, data)
	// if err != nil {
	// 	return nil, err
	// }
	// r := randomSrc{
	// 	o:   o,
	// 	rng: rng,
	// 	buf: newScrambler(data, o.totalSize, rng),
	// 	obj: Object{
	// 		Reader:      nil,
	// 		Name:        "",
	// 		ContentType: "application/octet-stream",
	// 		Size:        0,
	// 	},
	// }
	// r.obj.setPrefix(o)
	// return &r, nil
}

func (c *plainSrc) Object() *Object {
	// opts := c.o.csv
	dst := c.buf.data[:0]
	// for i := 0; i < len(dst); i++ {
	// 	dst[i] = byte(97)
	// }
	c.obj.Size = c.o.getSize(c.rng)
	// dst := [c.obj.Size]byte
	build := make([]byte, c.obj.Size)
	_, err := cRand.Read(build)
	// randASCIIBytes(build[:c.obj.Size], c.rng)
	if err != nil {
		fmt.Println("error:", err)
		// return
	}
	// for i := 0; i < int(c.obj.Size); i++ {
	dst = append(dst, build...)
	splitLength := len(dst)/4

	for i := splitLength; i < len(dst); i++ {
		dst[i] = dst[i%splitLength]
	}

	// }
	// for i := 0; i < opts.rows; i++ {
	// 	for j := 0; j < opts.cols; j++ {
	// 		fieldLen := 1 + opts.minLen
	// 		if opts.minLen != opts.maxLen {
	// 			fieldLen += c.rng.Intn(opts.maxLen - opts.minLen)
	// 		}
	// 		build := c.builder[:fieldLen]
	// 		randASCIIBytes(build[:fieldLen-1], c.rng)
	// 		build[fieldLen-1] = opts.comma
	// 		if j == opts.cols-1 {
	// 			build[fieldLen-1] = '\n'
	// 		}
	// 		dst = append(dst, build...)
	// 	}
	// }
	c.buf.data = dst
	c.obj.Reader = c.buf.Reset(0)
	var nBuf [16]byte
	randASCIIBytes(nBuf[:], c.rng)
	c.obj.setName(string(nBuf[:]) + ".txt")
	return &c.obj
}

func (r *randomSrc) Object() *Object {
	atomic.AddUint64(&r.counter, 1)
	var nBuf [16]byte
	randASCIIBytes(nBuf[:], r.rng)
	r.obj.Size = r.o.getSize(r.rng)
	r.obj.setName(fmt.Sprintf("%d.%s.rnd", atomic.LoadUint64(&r.counter), string(nBuf[:])))

	// Reset scrambler
	r.obj.Reader = r.buf.Reset(r.obj.Size)
	return &r.obj
}

func (r *randomSrc) String() string {
	if r.o.randSize {
		return fmt.Sprintf("Random data; random size up to %d bytes", r.o.totalSize)
	}
	return fmt.Sprintf("Random data; %d bytes total", r.buf.want)
}

func (r *plainSrc) String() string {
	if r.o.randSize {
		return fmt.Sprintf("Random data; random size up to %d bytes", r.o.totalSize)
	}
	return fmt.Sprintf("Random data; %d bytes total", r.buf.want)
}

func (r *plainSrc) Prefix() string {
	return r.obj.Prefix
}

func (r *randomSrc) Prefix() string {
	return r.obj.Prefix
}
