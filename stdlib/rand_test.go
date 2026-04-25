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
	var seed int64 = 1234
	r := rand.New(rand.NewSource(seed))

	module(t, "rand").call("seed", seed).expect(tengo.UndefinedValue)
	module(t, "rand").call("int").expect(r.Int63())
	module(t, "rand").call("float").expect(r.Float64())
	module(t, "rand").call("intn", 111).expect(r.Int63n(111))
	module(t, "rand").call("exp_float").expect(r.ExpFloat64())
	module(t, "rand").call("norm_float").expect(r.NormFloat64())
	module(t, "rand").call("perm", 10).expect(r.Perm(10))

	buf1 := make([]byte, 10)
	buf2 := &tengo.Bytes{Value: make([]byte, 10)}
	readBytes(r, buf1)
	module(t, "rand").call("read", buf2).expect(len(buf1))
	require.Equal(t, buf1, buf2.Value)

	seed = 9191
	r = rand.New(rand.NewSource(seed))
	randObj := module(t, "rand").call("rand", seed)
	randObj.call("seed", seed).expect(tengo.UndefinedValue)
	randObj.call("int").expect(r.Int63())
	randObj.call("float").expect(r.Float64())
	randObj.call("intn", 111).expect(r.Int63n(111))
	randObj.call("exp_float").expect(r.ExpFloat64())
	randObj.call("norm_float").expect(r.NormFloat64())
	randObj.call("perm", 10).expect(r.Perm(10))

	buf1 = make([]byte, 12)
	buf2 = &tengo.Bytes{Value: make([]byte, 12)}
	readBytes(r, buf1)
	randObj.call("read", buf2).expect(len(buf1))
	require.Equal(t, buf1, buf2.Value)
}
