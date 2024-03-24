package filerotate

import (
	"container/list"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FileRotate rotate files when writing
type FileRotate struct {
	opt    option
	dir    string
	fname  string
	stdlog *log.Logger

	lastRotateFormat string
	nextSplitNum     int

	writer *os.File
	fsize  int64
	files  *list.List

	closed int32
	wg     sync.WaitGroup
	err    error
}

// rotateItem
type rotateItem struct {
	rotateTime int64
	rotateNum  int
	fname      string
}

// parseRotateItem loads existing files，所有现存的备份日志文件进行排序，并返回，主要，info.log日志不在list中
func parseRotateItem(dir, fname, rotateFormat string) (*list.List, error) {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// parse exists log file filename
	parse := func(s string) (rt rotateItem, err error) {
		// remove filename and left "." error.log.2018-09-12.001 -> 2018-09-12.001
		rt.fname = s
		s = strings.TrimLeft(s[len(fname):], ".")
		seqs := strings.Split(s, ".")
		var t time.Time
		// TODO 向前兼容，新版本只会存在2018-09-12.001这样的格式
		switch len(seqs) {
		case 2:
			if rt.rotateNum, err = strconv.Atoi(seqs[1]); err != nil {
				return
			}
			fallthrough
		case 1:
			if t, err = time.Parse(rotateFormat, seqs[0]); err != nil {
				return
			}
			rt.rotateTime = t.Unix()
		}
		return
	}

	var items []rotateItem
	for _, fi := range fis {
		if strings.HasPrefix(fi.Name(), fname) && fi.Name() != fname {
			rt, err := parse(fi.Name())
			if err != nil {
				continue
			}
			items = append(items, rt)
		}
	}

	//从旧到新进行排列
	sort.Slice(items, func(i, j int) bool {
		if items[i].rotateTime == items[j].rotateTime {
			return items[i].rotateNum < items[j].rotateNum
		}
		return items[i].rotateTime < items[j].rotateTime
	})
	l := list.New()

	for _, item := range items {
		l.PushBack(item)
	}
	return l, nil
}

// New FileWriter A FileWriter is safe for use by multiple goroutines simultaneously.
func New(fpath string, fns ...Option) (*FileRotate, error) {
	opt := defaultOption
	for _, fn := range fns {
		fn(&opt)
	}

	fname := filepath.Base(fpath)
	if fname == "" {
		return nil, fmt.Errorf("filename can't empty")
	}
	dir := filepath.Dir(fpath)
	fi, err := os.Stat(dir)
	if err == nil && !fi.IsDir() {
		return nil, fmt.Errorf("%s already exists and not a directory", dir)
	}
	if os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s error: %s", dir, err.Error())
		}
	}

	stdlog := log.New(os.Stderr, "flog ", log.LstdFlags)

	files, err := parseRotateItem(dir, fname, opt.RotateFormat)
	if err != nil {
		// set files a empty list
		files = list.New()
		stdlog.Printf("parseRotateItem error: %s", err)
	}

	lastRotateFormat := time.Now().Format(opt.RotateFormat)
	var nextSplitNum int
	if files.Len() > 0 {
		rt := files.Back().Value.(rotateItem)
		//  check contains is mush easy than compared with timestamp
		if strings.Contains(rt.fname, lastRotateFormat) {
			nextSplitNum = rt.rotateNum + 1
		}
	}

	fr := &FileRotate{
		opt:    opt,
		dir:    dir,
		fname:  fname,
		stdlog: stdlog,

		nextSplitNum:     nextSplitNum,
		lastRotateFormat: lastRotateFormat,

		files:  files,
		writer: nil,
	}

	if err := fr.reset(fpath); err != nil {
		return nil, fmt.Errorf("failed to reset current file to %s: %s", fpath, err)
	}

	fr.wg.Add(1)

	go fr.daemon()

	return fr, nil
}

// reset open fpath and set the handler to current file
func (f *FileRotate) reset(fpath string) error {
	// close current file first
	if f.writer != nil {
		f.writer.Close()
	}

	// open new file
	fp, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fi, err := fp.Stat()
	if err != nil {
		return err
	}

	f.writer = fp
	f.fsize = fi.Size()
	return nil
}

// rotate rotate current file to old file, reset current file  to new file
func (f *FileRotate) rotate(fpath string) error {
	// first init
	if f.writer == nil {
		return f.reset(fpath)
	}

	formatFname := func(format string, num int) string {
		return fmt.Sprintf("%s.%s.%03d", f.fname, format, num)
	}

	// rename file
	fname := formatFname(f.lastRotateFormat, f.nextSplitNum)
	oldpath := filepath.Join(f.dir, f.fname)
	newpath := filepath.Join(f.dir, fname)
	if err := os.Rename(oldpath, newpath); err != nil {
		return err
	}

	if err := f.reset(fpath); err != nil {
		return err
	}

	f.files.PushBack(rotateItem{fname: fname /*rotateNum: f.lastSplitNum, rotateTime: t.Unix() unnecessary*/})

	return nil
}

// Write write data to iobuf
func (f *FileRotate) Write(p []byte) (n int, err error) {
	if f.err != nil {
		return 0, f.err
	}
	// atomic is not necessary
	if atomic.LoadInt32(&f.closed) == 1 {
		f.stdlog.Printf("%s", p)
		return 0, fmt.Errorf("filewriter already closed")
	}
	n, err = f.writer.Write(p)
	f.err = err
	f.fsize += int64(n)
	f.err = f.checkRotate()
	return
}

func (f *FileRotate) daemon() {
	tk := time.NewTicker(100 * time.Millisecond)
	defer tk.Stop()
	for {
		select {
		case <-tk.C:
			if err := f.checkDelete(); err != nil {
				f.stdlog.Printf("remove file error: %s", err)
			}
		}
		if atomic.LoadInt32(&f.closed) != 1 {
			continue
		}
		break
	}
	f.wg.Done()
}

// Close close file writer
func (f *FileRotate) Close() error {
	atomic.StoreInt32(&f.closed, 1)
	f.wg.Wait()
	if f.writer != nil {
		f.writer.Close()
	}
	return nil
}

// checkDelete delete files which are beyond count limit
func (f *FileRotate) checkDelete() error {
	if f.opt.MaxFile != 0 {
		for f.files.Len() > f.opt.MaxFile {
			rt := f.files.Remove(f.files.Front()).(rotateItem)
			fpath := filepath.Join(f.dir, rt.fname)
			if err := os.Remove(fpath); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkRotate rotate files if necessary
func (f *FileRotate) checkRotate() error {
	format := time.Now().Format(f.opt.RotateFormat)

	// 滚动条件：时间 or 大小
	if format != f.lastRotateFormat || (f.opt.MaxSize != 0 && f.fsize > f.opt.MaxSize) {
		if err := f.rotate(filepath.Join(f.dir, f.fname)); err != nil {
			return fmt.Errorf("failed to rotate log %v", err)
		}

		if format != f.lastRotateFormat {
			f.lastRotateFormat = format
			f.nextSplitNum = 0
		} else {
			f.nextSplitNum++
		}
	}
	return nil
}
