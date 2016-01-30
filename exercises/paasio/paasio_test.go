package paasio

import (
	"bytes"
	"crypto/rand"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestVersion identifies the API tested by the test program.
const testVersion = 1

func TestMultiThreaded(t *testing.T) {
	if TestVersion != testVersion {
		t.Errorf("Found TestVersion = %v, want %v.", TestVersion, testVersion)
	}
	mincpu := 2
	minproc := 2
	ncpu := runtime.NumCPU()
	if ncpu < mincpu {
		t.Fatalf("at least %d cpu cores are required", mincpu)
	}
	nproc := runtime.GOMAXPROCS(0)
	if nproc < minproc {
		t.Errorf("at least %d threads are required; rerun the tests", minproc)
		t.Errorf("")
		t.Errorf("\tgo test -cpu %d ...", minproc)
	}
}

// this test could be improved to test that error conditions are preserved.
func testWrite(t *testing.T, writer func(io.Writer) WriteCounter) {
	for i, test := range []struct {
		writes []string
	}{
		{nil},
		{[]string{""}},
		{[]string{"I", " ", "never met ", "", "a gohper"}},
	} {
		var buf bytes.Buffer
		buft := writer(&buf)
		for _, s := range test.writes {
			n, err := buft.Write([]byte(s))
			if err != nil {
				t.Errorf("test %d: Write(%q) unexpected error: %v", i, s, err)
				continue
			}
			if n != len(s) {
				t.Errorf("test %d: Write(%q) unexpected number of bytes written: %v", i, s, n)
				continue
			}
		}
		out := buf.String()
		if out != strings.Join(test.writes, "") {
			t.Errorf("test %d: unexpected content in underlying writer: %q", i, out)
		}
	}
}

func TestWriteWriter(t *testing.T) {
	testWrite(t, NewWriteCounter)
}

// this test could be improved to test exact number of operations as well as
// ensure that error conditions are preserved.
func testRead(t *testing.T, reader func(io.Reader) ReadCounter) {
	chunkLen := 10 << 20 // 10MB
	orig := make([]byte, 10<<20)
	_, err := rand.Read(orig)
	if err != nil {
		t.Fatalf("error reading random data")
	}
	buf := bytes.NewBuffer(orig)
	rc := reader(buf)
	var obuf bytes.Buffer
	ncopy, err := io.Copy(&obuf, rc)
	if err != nil {
		t.Fatalf("error reading: %v", err)
	}
	if ncopy != int64(chunkLen) {
		t.Fatalf("copied %d bytes instead of %d", ncopy, chunkLen)
	}
	if string(orig) != obuf.String() {
		t.Fatalf("unexpected output from Read()")
	}
	n, nops := rc.ReadCount()
	if n != int64(chunkLen) {
		t.Fatalf("reported %d bytes read instead of %d", n, chunkLen)
	}
	if nops < 2 {
		t.Fatalf("unexpected number of reads: %v", nops)
	}
}

func TestReadReader(t *testing.T) {
	testRead(t, NewReadCounter)
}

func testReadTotal(t *testing.T, rc ReadCounter) {
	numGo := 8000
	numBytes := 50
	totalBytes := int64(numGo) * int64(numBytes)
	p := make([]byte, numBytes)

	t.Logf("Calling Read() for %d*%d=%d bytes", numGo, numBytes, totalBytes)
	wg := new(sync.WaitGroup)
	wg.Add(numGo)
	start := make(chan struct{})
	for i := 0; i < numGo; i++ {
		go func() {
			<-start
			rc.Read(p)
			wg.Done()
		}()
	}
	close(start)

	wg.Wait()
	n, nops := rc.ReadCount()
	if n != totalBytes {
		t.Errorf("expected %d bytes read; %d bytes reported", totalBytes, n)
	}
	if nops != numGo {
		t.Errorf("expected %d read operations; %d operations reported", numGo, nops)
	}
}

func TestReadTotalReader(t *testing.T) {
	var r nopReader
	testReadTotal(t, NewReadCounter(r))
}

func testWriteTotal(t *testing.T, wt WriteCounter) {
	numGo := 8000
	numBytes := 50
	totalBytes := int64(numGo) * int64(numBytes)
	p := make([]byte, numBytes)

	t.Logf("Calling Write() with %d*%d=%d bytes", numGo, numBytes, totalBytes)
	wg := new(sync.WaitGroup)
	wg.Add(numGo)
	start := make(chan struct{})
	for i := 0; i < numGo; i++ {
		go func() {
			<-start
			wt.Write(p)
			wg.Done()
		}()
	}
	close(start)

	wg.Wait()
	n, nops := wt.WriteCount()
	if n != totalBytes {
		t.Errorf("expected %d bytes written; %d bytes reported", totalBytes, n)
	}
	if nops != numGo {
		t.Errorf("expected %d write operations; %d operations reported", numGo, nops)
	}
}

func TestWriteTotalWriter(t *testing.T) {
	var w nopWriter
	testWriteTotal(t, NewWriteCounter(w))
}

type nopWriter struct{ error }

func (w nopWriter) Write(p []byte) (int, error) {
	time.Sleep(1)
	if w.error != nil {
		return 0, w.error
	}
	return len(p), nil
}

type nopReader struct{ error }

func (r nopReader) Read(p []byte) (int, error) {
	time.Sleep(1)
	if r.error != nil {
		return 0, r.error
	}
	return len(p), nil
}
