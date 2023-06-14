package filerotate

import (
	"fmt"
	"strings"
)

// RotateFormat
const (
	RotateDaily = "2006-01-02"
)

var defaultOption = option{
	RotateFormat: RotateDaily,
	MaxSize:      1 << 30,
	BufSize:      4096,
}

type option struct {
	RotateFormat string
	MaxFile      int
	MaxSize      int64
	BufSize      int
}

// Option filewriter option
type Option func(opt *option)

// RotateFormat e.g 2006-01-02 meaning rotate log file every day.
// NOTE: format can't contain ".", "." will cause panic ヽ(*。>Д<)o゜.
func RotateFormat(format string) Option {
	if strings.Contains(format, ".") {
		panic(fmt.Sprintf("rotate format can't contain '.' format: %s", format))
	}
	return func(opt *option) {
		opt.RotateFormat = format
	}
}

// MaxFile default 999, 0 meaning unlimit.
// TODO: don't create file list if MaxSize is unlimt.
func MaxFile(n int) Option {
	return func(opt *option) {
		opt.MaxFile = n
	}
}

// MaxSize set max size for single log file,
// defult 1GB, 0 meaning unlimit.
func MaxSize(n int64) Option {
	return func(opt *option) {
		opt.MaxSize = n
	}
}
