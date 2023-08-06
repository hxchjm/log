package log

import (
	"bytes"
	"context"
	"github.com/hxchjm/log/env"
	stdlog "log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hxchjm/log/core"
)

const (
	_agentTimeout        = 20 * time.Millisecond
	_defaultChan         = 2048
	_defaultSocketBuffer = 200 * 1024 // linux core.net.wmem_default 212992
	_defaultAgentConfig  = "unixpacket:///var/run/lancer/collector_tcp.sock?timeout=100ms&chan=1024"
	_defaultAgentPath    = "unixgram:///var/run/log-agent/collector.sock?timeout=100ms&chan=1024?timeout=100ms&chan=1024"
)

var (
	_dialBackoff = &BackoffConfig{
		BaseDelay: 100 * time.Millisecond,
		MaxDelay:  time.Second,
		Factor:    1.2,
		Jitter:    0.2,
	}
	_mergeWait          = 1 * time.Second
	_logSeparator       = []byte("\u0001")
	_logSeparatorLength = len(_logSeparator)

	_defaultTaskIDs = map[string]string{
		env.DeployEnvFat1:   "000069",
		env.DeployEnvUat:    "000069",
		env.DeployEnvAvalon: "000069",
		env.DeployEnvPre:    "000161",
		env.DeployEnvProd:   "000161",
	}
)

// AgentHandler agent struct.
type AgentHandler struct {
	c         *AgentConfig
	msgs      chan []core.Field
	waiter    sync.WaitGroup
	pool      sync.Pool
	enc       core.Encoder
	batchSend bool
	conn      net.Conn
	drops     int
	ctx       context.Context
	cancel    context.CancelFunc
}

// AgentConfig agent config.
type AgentConfig struct {
	TaskID          string
	Buffer          int
	Proto           string        `json:"network"`
	Addr            string        `json:"address"`
	Chan            int           `json:"chan"`
	Timeout         time.Duration `json:"timeout"`
	SockWriteBuffer int           `json:"sockwritebuf"`
}

// NewAgent a Agent.
func NewAgent(ac *AgentConfig) (a *AgentHandler) {
	if ac == nil {
		ac = parseDSN(_agentDSN)
	}
	if ac.TaskID == "" {
		ac.TaskID = _defaultTaskIDs[env.DeployEnv]
	}
	a = &AgentHandler{
		c: ac,
		enc: core.NewJSONEncoder(core.EncoderConfig{
			EncodeTime:     core.EpochTimeEncoder,
			EncodeDuration: core.SecondsDurationEncoder,
		}, core.NewBuffer(0)),
	}
	a.pool.New = func() interface{} {
		return make([]core.Field, 0, 16)
	}
	if ac.Chan == 0 {
		ac.Chan = _defaultChan
	}
	a.msgs = make(chan []core.Field, ac.Chan)
	if ac.Timeout == 0 {
		ac.Timeout = _agentTimeout
	}
	if ac.Buffer == 0 {
		ac.Buffer = 100
	}

	if ac.SockWriteBuffer == 0 {
		ac.SockWriteBuffer = _defaultSocketBuffer
	}

	a.waiter.Add(1)

	// set fixed k/v into enc buffer
	KVString(_appID, c.Family).AddTo(a.enc)
	KVString(_deplyEnv, env.DeployEnv).AddTo(a.enc)
	KVString(_instanceID, c.Host).AddTo(a.enc)
	//KVString(_zone, env.Zone).AddTo(a.enc)

	if a.c.Proto == "unixpacket" {
		a.batchSend = true
	}

	a.ctx, a.cancel = context.WithCancel(context.Background())

	go a.writeproc()
	return
}

func (h *AgentHandler) data() []core.Field {
	return h.pool.Get().([]core.Field)
}

func (h *AgentHandler) free(f []core.Field) {
	f = f[0:0]
	h.pool.Put(f)
}

// Log log to udp statsd daemon.
func (h *AgentHandler) Log(ctx context.Context, lv Level, args ...D) {
	select {
	case <-h.ctx.Done():
		h.drops++
		if h.drops == 1 || h.drops%h.c.Buffer == 0 {
			stdlog.Printf("LogHandler is closed. drop %d messages so far\n", h.drops)
		}
		return
	default:
	}

	if args == nil {
		return
	}
	f := h.data()
	for i := range args {
		f = append(f, args[i])
	}
	if t, ok := FromContext(ctx); ok {
		f = append(f, KVString(_tid, t))
	}
	if env.DeployEnv != "" {
		f = append(f, KVString(_deplyEnv, env.DeployEnv))
	}
	//if t, ok := trace.FromContext(ctx); ok {
	//	f = append(f, KVString(_tid, t.TraceID()), KVString(_span, t.SpanID()))
	//}
	//if caller := metadata.String(ctx, metadata.Caller); caller != "" {
	//	f = append(f, KVString(_caller, caller))
	//}
	//if color := metadata.String(ctx, metadata.Color); color != "" {
	//	f = append(f, KVString(_color, color))
	//}
	//if env.Color != "" {
	//	f = append(f, KVString(_envColor, env.Color))
	//}
	//if cluster := metadata.String(ctx, metadata.Cluster); cluster != "" {
	//	f = append(f, KVString(_cluster, cluster))
	//}
	//if metadata.String(ctx, metadata.Mirror) != "" {
	//	f = append(f, KV(_mirror, true))
	//}
	select {
	case h.msgs <- f:
	default:
		h.drops++
		if h.drops == 1 || h.drops%h.c.Buffer == 0 {
			stdlog.Printf("writeproc queue full. drop %d messages so far\n", h.drops)
		}
	}
}

// dialWithRetry connect to remote with retry
func (h *AgentHandler) dialWithRetry() error {
	var err error
	h.conn, err = net.DialTimeout(h.c.Proto, h.c.Addr, time.Duration(h.c.Timeout))
	if err != nil {
		stdlog.Printf("net.DialTimeout(%s:%s) error(%v)\n", h.c.Proto, h.c.Addr, err)
		return err
	}
	if unixConn, ok := h.conn.(*net.UnixConn); ok {
		_ = unixConn.SetWriteBuffer(h.c.SockWriteBuffer)
	}
	return nil
}

// sendBytes send bytes as one package
func (h *AgentHandler) sendBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	// do not send buffer > SockWriteBuffer
	if len(b) > h.c.SockWriteBuffer {
		stdlog.Printf("buf.Write(%d bytes) message too long\n", len(b))
		return
	}

	// conn.Write retry only with net.Error
	retries := 0
	for {
		retries++
		if h.conn == nil {
			// when connection closed and ctx is done, give up to send
			select {
			case <-h.ctx.Done():
				return
			default:
				if err := h.dialWithRetry(); err != nil {
					time.Sleep(_dialBackoff.Backoff(retries))
					continue
				}
				retries = 0
			}
		}
		if _, err := h.conn.Write(b); err != nil {
			stdlog.Printf("conn.Write(%d bytes) error(%v)\n", len(b), err)
			h.conn.Close()
			h.conn = nil
			if _, ok := err.(net.Error); ok {
				time.Sleep(_dialBackoff.Backoff(retries))
				continue
			}
		}
		break
	}
}

// sendBatchBuffer determine whether should split buffer
func (h *AgentHandler) sendBatchBuffer(buf *core.Buffer) {
	bufLength := buf.Len()
	if bufLength == 0 {
		return
	}
	defer buf.Reset()
	// split _logSeparator for not batchSend
	if !h.batchSend {
		for _, b := range bytes.Split(buf.Bytes(), _logSeparator) {
			h.sendBytes(b)
		}
		return
	}

	b := buf.Bytes()
	index := bytes.LastIndex(b[:bufLength-_logSeparatorLength], _logSeparator)

	// batchSend for buffer size < SockWriteBuffer or only one buffer
	if bufLength <= h.c.SockWriteBuffer || index == -1 {
		h.sendBytes(b)
		return
	}

	// batchSend for buffers total size > SockWriteBuffer
	index += _logSeparatorLength
	h.sendBytes(b[:index])
	// also send the last buffer with _logSeparator as batchSend
	h.sendBytes(b[index:])
}

// writeproc collect data and write into connection.
func (h *AgentHandler) writeproc() {
	var (
		count int
		buf   = core.NewBuffer(h.c.SockWriteBuffer)
		//taskID      = []byte(h.c.TaskID)
		tick        = time.NewTicker(_mergeWait)
		channelOpen = true
	)
	defer h.waiter.Done()
	defer h.sendBatchBuffer(buf)
	defer tick.Stop()

	for {
		select {
		case <-h.ctx.Done():
			channelOpen = false
		default:
		}

		select {
		case d := <-h.msgs:
			//_, _ = buf.Write(taskID)
			//_, _ = buf.Write([]byte(strconv.FormatInt(time.Now().UnixNano()/1e6, 10)))
			_ = h.enc.Encode(buf, d...)
			h.free(d)
			_, _ = buf.Write(_logSeparator)
			count++
			if count < h.c.Buffer && buf.Len() <= h.c.SockWriteBuffer { // batchSend with limit size
				continue
			}
		case <-tick.C:
		}
		if buf.Len() == 0 && !channelOpen {
			return
		}
		h.sendBatchBuffer(buf)
		count = 0
	}
}

// Close close the connection.
func (h *AgentHandler) Close() (err error) {
	h.cancel()
	h.waiter.Wait()
	if h.conn != nil {
		_ = h.conn.Close()
		h.conn = nil
	}
	return nil
}

// SetFormat not use.
func (h *AgentHandler) SetFormat(string) {
	// discard setformat
}

// parseDSN parse log agent dsn.
// unixgram:///var/run/log-agent/collector.sock?timeout=100ms&chan=1024
func parseDSN(dsn string) *AgentConfig {
	u, _ := url.Parse(dsn)
	v := u.Query()
	chanNum, _ := strconv.Atoi(v.Get("chan"))
	timeout, _ := time.ParseDuration(v.Get("timeout"))
	//timeout, _ := strconv.Atoi(v.Get("timeout"))
	sockWriteBuf, _ := strconv.Atoi(v.Get("sockwritebuf"))

	return &AgentConfig{
		Proto:           u.Scheme,
		Addr:            u.Path,
		Chan:            chanNum,
		Timeout:         timeout,
		SockWriteBuffer: sockWriteBuf,
	}
}
