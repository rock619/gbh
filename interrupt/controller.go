package interrupt

import (
	"sync"

	"github.com/rock619/gbh/memorybus"
	"github.com/rock619/gbh/register"
)

type Requester interface {
	Request(interrupt Interrupt)
}

type Interrupt struct {
	bit         uint8
	HandlerAddr uint16
}

var (
	VBlank  = Interrupt{bit: 0, HandlerAddr: 0x40}
	LCDStat = Interrupt{bit: 1, HandlerAddr: 0x48}
	Timer   = Interrupt{bit: 2, HandlerAddr: 0x50}
	Serial  = Interrupt{bit: 3, HandlerAddr: 0x58}
	Joypad  = Interrupt{bit: 4, HandlerAddr: 0x60}
)

var Interrupts = []Interrupt{
	VBlank,
	LCDStat,
	Timer,
	Serial,
	Joypad,
}

type Controller struct {
	memorybus.Memory

	mu       sync.Mutex
	ime      bool  // Interrupt Master Enable flag
	imeDelay int   // Indicates if IME should be enabled after the next instruction (used for EI instruction)
	ie       uint8 // Interrupt Enable register (0xFFFF)
	if_      uint8 // Interrupt Flag register (0xFF0F)
}

func New() *Controller {
	return &Controller{}
}

func (c *Controller) Read(addr uint16) uint8 {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch addr {
	case register.IE:
		return c.ie
	case register.IF:
		return 0b11100000 | c.if_ // unused bits read as 1
	default:
		return 0xFF
	}
}

func (c *Controller) Write(addr uint16, value uint8) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch addr {
	case register.IE:
		c.ie = value
	case register.IF:
		c.if_ = value & 0b11111 // Only lower 5 bits are writable
	default:
	}
}

func (c *Controller) IME() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ime
}

func (c *Controller) DisableIME() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ime = false
	c.imeDelay = 0
}

func (c *Controller) RequestIME() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ime {
		c.imeDelay = 2
	}
}

func (c *Controller) EnableIME() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ime = true
	c.imeDelay = 0
}

func (c *Controller) Step() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.imeDelay > 0 {
		c.imeDelay--
		if c.imeDelay == 0 {
			c.ime = true
		}
	}
}

func (c *Controller) IE() uint8 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ie
}

func (c *Controller) IF() uint8 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.if_
}

func (c *Controller) HasPending() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ie&c.if_&0x1F != 0
}

func (c *Controller) ResetIF(interrupt Interrupt) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.if_ &^= 1 << interrupt.bit
}

func (c *Controller) RequestedInterrupt() (Interrupt, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.ime {
		return Interrupt{}, false
	}
	for _, interrupt := range Interrupts {
		mask := uint8(1 << interrupt.bit)
		if c.ie&c.if_&mask != 0 {
			return interrupt, true
		}
	}
	return Interrupt{}, false
}

func (c *Controller) Request(interrupt Interrupt) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.if_ |= 1 << interrupt.bit
}
