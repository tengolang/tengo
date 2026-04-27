package stdlib_test

import (
	"math/rand"
	"testing"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/require"
)

// readBytes replicates the 7-bytes-per-Int63 algorithm used by the rand
// module's "read" function, without calling the deprecated (*rand.Rand).Read.
func readBytes(r *rand.Rand, p []byte) {
	var val int64
	var pos int8
	for i := range p {
		if pos == 0 {
			val = r.Int63()
			pos = 7
		}
		p[i] = byte(val)
		val >>= 8
		pos--
	}
}

func TestRand(t *testing.T) {
	// rand.new(seed) — reproducible, isolated instance
	var seed int64 = 1234
	r := rand.New(rand.NewSource(seed))
	rnd := module(t, "rand").call("new", seed)

	rnd.call("int").expect(r.Int63())
	rnd.call("float").expect(r.Float64())
	rnd.call("intn", 111).expect(r.Int63n(111))
	rnd.call("exp_float").expect(r.ExpFloat64())
	rnd.call("norm_float").expect(r.NormFloat64())
	rnd.call("perm", 10).expect(r.Perm(10))

	buf1 := make([]byte, 10)
	buf2 := &tengo.Bytes{Value: make([]byte, 10)}
	readBytes(r, buf1)
	rnd.call("read", buf2).expect(len(buf1))
	require.Equal(t, buf1, buf2.Value)

	// seed() on an instance resets its sequence
	seed2 := int64(9191)
	r2 := rand.New(rand.NewSource(seed2))
	rnd.call("seed", seed2).expect(tengo.UndefinedValue)
	rnd.call("int").expect(r2.Int63())

	// rand.new() without seed — non-deterministic, just verify no error
	module(t, "rand").call("new").noError()

	// global functions — non-deterministic, smoke-test only
	module(t, "rand").call("int").noError()
	module(t, "rand").call("float").noError()
	module(t, "rand").call("intn", 100).noError()
	module(t, "rand").call("exp_float").noError()
	module(t, "rand").call("norm_float").noError()
	module(t, "rand").call("perm", 5).noError()

	buf3 := &tengo.Bytes{Value: make([]byte, 8)}
	module(t, "rand").call("read", buf3).expect(8)
}
