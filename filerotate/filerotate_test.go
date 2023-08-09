package filerotate

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
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

func TestParseRotate(t *testing.T) {
	touch := func(dir, name string) {
		os.MkdirAll(dir, 0755)
		fp, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE, 0644)
		if err != nil {
			t.Fatal(err)
		}
		fp.Close()
	}
	dir := filepath.Join(logdir, "test-parse-rotate")
	names := []string{"info.log.2018-11-11", "info.log.2018-11-11.001", "info.log.2018-11-11.002", "info.log." + time.Now().Format("2006-01-02") + ".005"}
	for _, name := range names {
		touch(dir, name)
	}
	l, err := parseRotateItem(dir, "info.log", "2006-01-02")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, len(names), l.Len())

	rt := l.Back().Value.(rotateItem)

	assert.Equal(t, 5, rt.rotateNum)
}

func TestRotateExists(t *testing.T) {
	dir := filepath.Join(logdir, "test-rotate-exists")
	names := []string{
		"info.log." + time.Now().Format("2006-01-02") + ".005",
		"info.log." + time.Now().Format("2006-01-02") + ".006",
	}
	for _, name := range names {
		touch(dir, name)
	}
	fw, err := New(logdir+"/test-rotate-exists/info.log",
		MaxSize(1024*1024),
		MaxFile(3),
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
	fis, err := ioutil.ReadDir(logdir + "/test-rotate-exists")
	if err != nil {
		t.Fatal(err)
	}
	var fnams []string
	for _, fi := range fis {
		fnams = append(fnams, fi.Name())
	}
	assert.Contains(t, fnams, "info.log."+time.Now().Format("2006-01-02")+".010")
}

func TestSizeRotate(t *testing.T) {
	fw, err := New(logdir+"/test-rotate/info.log",
		MaxSize(1024*1024),
	)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 10; i++ {
		for i := 0; i < 1024; i++ {
			_, err = fw.Write(data)
			if err != nil {
				t.Error(err)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	fw.Close()
	fis, err := ioutil.ReadDir(logdir + "/test-rotate")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(fis) > 5, "expect more than 5 file get %d", len(fis))
}

func TestMaxFile(t *testing.T) {
	fw, err := New(logdir+"/test-maxfile/info.log",
		MaxSize(1024*1024),
		MaxFile(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 10; i++ {
		for i := 0; i < 1024; i++ {
			_, err = fw.Write(data)
			if err != nil {
				t.Error(err)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	fw.Close()
	fis, err := ioutil.ReadDir(logdir + "/test-maxfile")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(fis) <= 2, fmt.Sprintf("expect 2 file get %d", len(fis)))
}

func TestMaxFile2(t *testing.T) {
	files := []string{
		"info.log.2018-12-01",
		"info.log.2018-12-02",
		"info.log.2018-12-03",
		"info.log.2018-12-04",
		"info.log.2018-12-05",
		"info.log.2018-12-05.001",
	}
	for _, file := range files {
		touch(logdir+"/test-maxfile2", file)
	}
	fw, err := New(logdir+"/test-maxfile2/info.log",
		MaxSize(1024*1024),
		MaxFile(3),
	)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	for i := 0; i < 10; i++ {
		for i := 0; i < 1024; i++ {
			_, err = fw.Write(data)
			if err != nil {
				t.Error(err)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	fw.Close()
	fis, err := ioutil.ReadDir(logdir + "/test-maxfile2")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(fis) == 4, fmt.Sprintf("expect 4 file get %d", len(fis)))
}

func TestFileWriter(t *testing.T) {
	fw, err := New("testlog/info.log")
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()
	_, err = fw.Write([]byte("Hello World!\n"))
	if err != nil {
		t.Error(err)
	}
}

func BenchmarkFileWriter(b *testing.B) {
	fw, err := New("testlog/bench/info.log",
		MaxSize(1024*1024*8),
	)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, err = fw.Write([]byte("Hello World!\n"))
		if err != nil {
			b.Error(err)
		}
	}
}

func TestRotateToday(t *testing.T) {
	convey.Convey("Top-level", t, func() {
		dir, err := ioutil.TempDir("/tmp", "filewriter_test")
		assert.NoError(t, err)
		defer os.RemoveAll(dir)

		var (
			maxFile int
			files   []string
		)

		convey.Convey("Test old same day", func() {
			maxFile = 3
			files = []string{
				"info.log.2018-12-05.002",
				"info.log.2018-12-05.003",
				"info.log.2018-12-05.004",
				"info.log.2018-12-05.005",
				"info.log.2018-12-05.006",
				"info.log.2018-12-05.007",
				"info.log.2018-12-05.008",
				"info.log.2018-12-05.009",
			}
		})
		convey.Convey("Test old different days", func() {
			maxFile = 3
			files = []string{
				"info.log.2018-12-06.006",
				"info.log.2018-12-06.007",
				"info.log.2018-12-06.008",
				"info.log.2018-12-06.009",
				"info.log.2018-12-05.002",
				"info.log.2018-12-05.003",
				"info.log.2018-12-05.004",
				"info.log.2018-12-05.005",
			}
		})
		convey.Convey("Test today", func() {
			maxFile = 3
			files = []string{}
			today := time.Now().Format("2006-01-02")
			for i := 1; i < 10; i++ {
				files = append(files, fmt.Sprintf("info.log.%s.00%d", today, i))
			}
		})
		convey.Convey("Test today + old days", func() {
			maxFile = 3
			files = []string{
				"info.log.2018-12-05",
				"info.log.2018-12-06",
				"info.log.2018-12-07",
				"info.log.2018-12-08",
				fmt.Sprintf("info.log.%s", time.Now().Format("2006-01-02")),
			}
		})
		convey.Convey("Test old days + split", func() {
			maxFile = 3
			files = []string{
				"info.log.2018-12-01",
				"info.log.2018-12-02",
				"info.log.2018-12-03",
				"info.log.2018-12-04",
				"info.log.2018-12-05",
				"info.log.2018-12-05.001",
			}
		})

		sort.Strings(files)
		for _, file := range files {
			touch(dir, file)
		}
		fw, err := New(dir+"/info.log",
			MaxSize(1024*1024),
			MaxFile(maxFile),
		)
		assert.NoError(t, err)
		defer fw.Close()
		// waite for clean
		time.Sleep(time.Millisecond * 200)

		fis, err := ioutil.ReadDir(dir)
		assert.NoError(t, err)

		actual := []string{}
		for _, fi := range fis {
			actual = append(actual, fi.Name())
		}

		expect := append([]string{"info.log"}, files[len(files)-maxFile:]...)
		assert.Equal(t, expect, actual)
	})
}

func TestWriteWhileRotate(t *testing.T) {
	var wg sync.WaitGroup

	fw, err := New("testlog/writewhilerotate/info.log",
		MaxSize(1025*10),
	)
	if err != nil {
		t.Fatal(err)
	}

	logsCount := 1000000

	var count int
	wg.Add(1)

	go func() {
		for i := 0; i < logsCount; i++ {
			n, err := fw.Write([]byte("hello world\n"))
			count += n
			if err != nil {
				t.Error(err)
			}
		}

		fw.Close()
		wg.Done()
	}()

	wg.Wait()
	assert.Equal(t, len([]byte("hello world\n"))*logsCount, count)

	// check write
	var countActual int
	fis, err := ioutil.ReadDir("testlog/writewhilerotate")
	if err != nil {
		t.Fatal(err)
	}

	for _, fi := range fis {
		file, _ := os.Open(filepath.Join("testlog/writewhilerotate", fi.Name()))
		fd := bufio.NewReader(file)
		for {
			_, err := fd.ReadString('\n')
			if err != nil {
				break
			}
			countActual++

		}
		file.Close()
	}

	assert.Equal(t, logsCount, countActual)
}
