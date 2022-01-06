package filewriter

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const logdir = "testlog"

func touch(dir, name string) {
	os.MkdirAll(dir, 0755)
	fp, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	fp.Close()
}

func TestMain(m *testing.M) {
	ret := m.Run()
	os.RemoveAll(logdir)
	os.Exit(ret)
}

func TestWrite(t *testing.T) {
	dir := filepath.Join(logdir, "testwrite")
	names := []string{
		"info.log." + time.Now().Format("2006-01-02") + ".005",
		"info.log." + time.Now().Format("2006-01-02") + ".006",
	}
	for _, name := range names {
		touch(dir, name)
	}
	fw, err := New(logdir+"/testwrite/info.log",
		MaxSize(1024*1024),
		MaxFile(3),
		BufSize(1024),
	)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1025)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 4; i++ {
		for i := 0; i < 1024; i++ {
			_, err = fw.Write(data)
			if err != nil {
				t.Error(err)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	fw.Close()

	fis, err := ioutil.ReadDir(logdir + "/testwrite")
	if err != nil {
		t.Fatal(err)
	}
	var fnams []string
	for _, fi := range fis {
		fnams = append(fnams, fi.Name())
	}
	assert.Contains(t, fnams, "info.log."+time.Now().Format("2006-01-02")+".010")

}

func TestFlushDuringWrite(t *testing.T) {
	fw, err := New(logdir+"/flush_during_write/info.log",
		MaxSize(1024*1024),
		MaxFile(3),
		BufSize(1024),
	)
	if err != nil {
		t.Fatal(err)
	}

	n, err := fw.Write([]byte("123"))
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second * 2)

	assert.Equal(t, 3, n)

	fis, err := ioutil.ReadDir(logdir + "/flush_during_write")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(fis))
	assert.Equal(t, int64(3), fis[0].Size())
}

func TestWriteShareBuffer(t *testing.T) {
	fw, err := New(logdir+"/write_share_buffer/info.log",
		MaxSize(0),
		BufSize(1024),
	)
	assert.NoError(t, err)
	buf := new(bytes.Buffer)
	length := len("long text") * 100000 / 2
	length += len("short") * 100000 / 2

	for i := 0; i < 100000; i++ {
		if i%2 == 0 {
			buf.WriteString("long text")
		} else {
			buf.WriteString("short")
		}
		buf.WriteTo(fw)
		buf.Reset()
	}
	fw.Close()
	content, err := ioutil.ReadFile(logdir + "/write_share_buffer/info.log")
	assert.NoError(t, err)
	assert.Equal(t, length, len(content))
}

func TestWriteConcurrency(t *testing.T) {
	fw, err := New(logdir+"/write_concurrency/info.log",
		MaxSize(0),
		BufSize(1024),
	)
	assert.NoError(t, err)
	longText := []byte("long text")
	shortText := []byte("short")
	concurrency := 100
	count := 1000

	exceptLength := (len(longText) + len(shortText)) * concurrency * count

	var length int64
	var wg sync.WaitGroup
	for x := 0; x < concurrency; x++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			var _length int
			for i := 0; i < count; i++ {
				n, _ := fw.Write(longText)
				_length += n
				time.Sleep(time.Millisecond * 1)
			}
			atomic.AddInt64(&length, int64(_length))
		}()
		go func() {
			defer wg.Done()
			var _length int
			for i := 0; i < count; i++ {
				n, _ := fw.Write(shortText)
				_length += n
				time.Sleep(time.Millisecond * 1)
			}
			atomic.AddInt64(&length, int64(_length))
		}()
	}
	wg.Wait()
	fw.Close()
	assert.Equal(t, exceptLength, int(length))

	content, err := ioutil.ReadFile(logdir + "/write_concurrency/info.log")
	assert.NoError(t, err)
	assert.Equal(t, exceptLength, len(content))
}
