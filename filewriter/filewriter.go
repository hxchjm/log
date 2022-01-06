package filewriter

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/hxchjm/log/filerotate"
)

// FileWriter create file log writer
type FileWriter struct {
	fpath      string
	writer     *bufio.Writer
	filerotate *filerotate.FileRotate
	opt        option
	ch         chan *bytes.Buffer
	stdlog     *log.Logger
	pool       *sync.Pool

	closed int32
	wg     sync.WaitGroup
}

// New FileWriter A FileWriter is safe for use by multiple goroutines simultaneously.
func New(fpath string, fns ...Option) (*FileWriter, error) {
	opt := defaultOption
	for _, fn := range fns {
		fn(&opt)
	}

	stdlog := log.New(os.Stderr, "flog ", log.LstdFlags)

	fw := &FileWriter{
		fpath:  fpath,
		opt:    opt,
		stdlog: stdlog,
		ch:     make(chan *bytes.Buffer, opt.ChanSize),
		pool:   &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }},
		writer: bufio.NewWriterSize(nil, opt.BufSize),
	}

	fw.wg.Add(1)
	err := fw.initFileRotate()
	if err != nil {
		return nil, err
	}
	go fw.daemon()

	return fw, nil
}

func (f *FileWriter) initFileRotate() (err error) {
	if f.writer != nil {
		f.writer.Flush()
	}

	if f.filerotate != nil {
		f.filerotate.Close()
	}

	if f.filerotate, err = filerotate.New(f.fpath, filerotate.MaxSize(f.opt.MaxSize), filerotate.MaxFile(f.opt.MaxFile),
		filerotate.RotateFormat(f.opt.RotateFormat)); err != nil {
		return err
	}

	f.writer.Reset(f.filerotate)
	return
}

// Write write data to log file, return write bytes is pseudo just for implement io.Writer.
func (f *FileWriter) Write(p []byte) (int, error) {
	// atomic is not necessary
	if atomic.LoadInt32(&f.closed) == 1 {
		f.stdlog.Printf("%s", p)
		return 0, fmt.Errorf("filewriter already closed")
	}
	// because write to file is asynchronousc,
	// copy p to internal buf prevent p be change on outside
	buf := f.getBuf()
	buf.Write(p)

	if f.opt.WriteTimeout == 0 {
		select {
		case f.ch <- buf:
			return len(p), nil
		default:
			// TODO: write discard log to to stdout?
			return 0, fmt.Errorf("log channel is full, discard log")
		}
	}

	// write log with timeout
	timeout := time.NewTimer(f.opt.WriteTimeout)
	select {
	case f.ch <- buf:
		return len(p), nil
	case <-timeout.C:
		// TODO: write discard log to to stdout?
		return 0, fmt.Errorf("log channel is full, discard log")
	}
}

func (f *FileWriter) daemon() {
	tk := time.NewTicker(time.Second * 1)
	for {
		select {
		case buf := <-f.ch:
			_, err := f.writer.Write(buf.Bytes())
			f.putBuf(buf)
			if err != nil {
				f.stdlog.Printf("failed to write bufio: %s", err)
				time.Sleep(time.Second * 1)
				if err := f.initFileRotate(); err != nil {
					f.stdlog.Printf("failed to initFileRotate %s", err)
				}
			}
		case <-tk.C:
			if f.writer.Buffered() != 0 {
				if err := f.writer.Flush(); err != nil {
					f.stdlog.Printf("failed to flush bufio: %s", err)
					time.Sleep(time.Second * 1)
					if err := f.initFileRotate(); err != nil {
						f.stdlog.Printf("failed to initFileRotate %s", err)
					}
				}
				continue
			}
		}
		if f.writer.Buffered() != 0 || atomic.LoadInt32(&f.closed) != 1 {
			continue
		}

		f.writer.Flush()
		f.filerotate.Close()
		break
	}
	f.wg.Done()
}

// Close close file writer
func (f *FileWriter) Close() error {
	atomic.StoreInt32(&f.closed, 1)
	f.wg.Wait()
	return nil
}

func (f *FileWriter) putBuf(buf *bytes.Buffer) {
	buf.Reset()
	f.pool.Put(buf)
}

func (f *FileWriter) getBuf() *bytes.Buffer {
	return f.pool.Get().(*bytes.Buffer)
}
