package log

import (
	"context"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"time"

	"github.com/hxchjm/log/core"
	//"go-common/library/net/metadata"
	//"go-common/library/net/trace"
)

const _ctxkey = "trace_id"

func FromContext(ctx context.Context) (t string, ok bool) {
	t, ok = ctx.Value(_ctxkey).(string)
	return
}

func addExtraField(ctx context.Context, fields map[string]interface{}) {
	if t, ok := FromContext(ctx); ok {
		fields[_tid] = t
		//fields[_span] = t.SpanID()
		//fields[_traceSampled] = t.IsSampled()
	}
	//if caller := metadata.String(ctx, metadata.Caller); caller != "" {
	//	fields[_caller] = caller
	//}
	//if color := metadata.String(ctx, metadata.Color); color != "" {
	//	fields[_color] = color
	//}
	//if env.Color != "" {
	//	fields[_envColor] = env.Color
	//}
	//if cluster := metadata.String(ctx, metadata.Cluster); cluster != "" {
	//	fields[_cluster] = cluster
	//}
	//fields[_deplyEnv] = env.DeployEnv
	//fields[_zone] = env.Zone
	fields[_appID] = c.Family
	fields[_instanceID] = c.Host
	//if metadata.String(ctx, metadata.Mirror) != "" {
	//	fields[_mirror] = true
	//}
}

// funcName get func name.
func funcName(skip int) (name string) {
	if _, file, lineNo, ok := runtime.Caller(skip); ok {
		return file + ":" + strconv.Itoa(lineNo)
	}
	return "unknown:0"
}

// toMap convert D slice to map[string]interface{} for legacy file and stdout.
func toMap(args ...D) map[string]interface{} {
	d := make(map[string]interface{}, 10+len(args))
	for _, arg := range args {
		switch arg.Type {
		case core.UintType, core.Uint64Type, core.IntTpye, core.Int64Type:
			d[arg.Key] = arg.Int64Val
		case core.StringType:
			d[arg.Key] = arg.StringVal
		case core.Float32Type:
			d[arg.Key] = math.Float32frombits(uint32(arg.Int64Val))
		case core.Float64Type:
			d[arg.Key] = math.Float64frombits(uint64(arg.Int64Val))
		case core.DurationType:
			d[arg.Key] = time.Duration(arg.Int64Val)
		default:
			d[arg.Key] = arg.Value
		}
	}
	return d
}

// BackoffConfig defines the parameters for the default backoff strategy.
type BackoffConfig struct {
	// MaxDelay is the upper bound of backoff delay.
	MaxDelay time.Duration

	// baseDelay is the amount of time to wait before retrying after the first
	// failure.
	BaseDelay time.Duration

	// factor is applied to the backoff after each retry.
	Factor float64

	// jitter provides a range to randomize backoff delays.
	Jitter float64
}

// Backoff returns the amount of time to wait before the next retry given
// the number of consecutive failures.
func (bc *BackoffConfig) Backoff(retries int) time.Duration {
	if retries == 0 {
		return bc.BaseDelay
	}
	backoff, max := float64(bc.BaseDelay), float64(bc.MaxDelay)
	for backoff < max && retries > 0 {
		backoff *= bc.Factor
		retries--
	}
	if backoff > max {
		backoff = max
	}
	// Randomize backoff delays so that if a cluster of requests start at
	// the same time, they won't operate in lockstep.
	backoff *= 1 + bc.Jitter*(rand.Float64()*2-1)
	if backoff < 0 {
		return 0
	}
	return time.Duration(backoff)
}
