package ticker

import "time"

// Ticker defines a ticker.
type Ticker struct {
	stop     chan struct{}
	tickerFn []func()
	d        time.Duration
}

type Config struct {
	TickerFns []func()
}

type ConfigFn func(*Config)

func WithTickerFn(tickerFns ...func()) ConfigFn {
	return func(c *Config) {
		c.TickerFns = append(c.TickerFns, tickerFns...)
	}
}

// New creates a new ticker.
func New(d time.Duration, fns ...ConfigFn) *Ticker {
	c := Config{}
	for _, fn := range fns {
		fn(&c)
	}

	j := &Ticker{
		d:        d,
		tickerFn: c.TickerFns,
	}
	return j
}

// Start starts the ticker.
// if tickerFns are passed, they will overwrite the previous passed in NewTicker call.
func (j *Ticker) Start(fns ...func()) {
	j.tickerFn = append(j.tickerFn, fns...)
	j.stop = make(chan struct{})
	go j.start()
}

func (j *Ticker) start() {
	t := time.NewTicker(j.d)
	defer t.Stop()

	for {
		j.execFns()

		select {
		case <-t.C:
		case <-j.stop:
			return
		}
	}
}

// Stop stops the ticker.
func (j *Ticker) Stop() {
	select {
	case j.stop <- struct{}{}:
	default:
	}
}

func (j *Ticker) execFns() {
	for _, fn := range j.tickerFn {
		if fn != nil {
			fn()
		}
	}
}
