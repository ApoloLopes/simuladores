// Package s7sim implementa um servidor S7comm (Siemens S7-300/400/1200/1500)
// em Go puro, suficiente para leitura/escrita de variáveis (DB, M, I, Q).
package s7sim

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sync"
)

// Códigos de área S7.
const (
	AreaInputs  byte = 0x81
	AreaOutputs byte = 0x82
	AreaMerkers byte = 0x83
	AreaDB      byte = 0x84
)

const maxPDU = 480

type areaKey struct {
	area byte
	db   uint16
}

// Server é um servidor S7 simulado.
type Server struct {
	Addr  string
	mu    sync.RWMutex
	areas map[areaKey][]byte
	ln    net.Listener
}

// New cria um servidor que escutará em addr ("ip:porta").
func New(addr string) *Server {
	return &Server{Addr: addr, areas: map[areaKey][]byte{}}
}

// RegisterArea registra uma área de memória (ex.: DB1 com 100 bytes).
func (s *Server) RegisterArea(area byte, db uint16, size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.areas[areaKey{area, db}] = make([]byte, size)
}

// ---- Helpers de acesso à DB (big-endian, como no Siemens) ----

func (s *Server) with(db uint16, off, n int, fn func(b []byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mem, ok := s.areas[areaKey{AreaDB, db}]
	if !ok || off < 0 || off+n > len(mem) {
		return fmt.Errorf("DB%d offset %d fora do intervalo", db, off)
	}
	fn(mem[off : off+n])
	return nil
}

func (s *Server) SetBool(db uint16, off int, bit uint, v bool) error {
	return s.with(db, off, 1, func(b []byte) {
		if v {
			b[0] |= 1 << bit
		} else {
			b[0] &^= 1 << bit
		}
	})
}

func (s *Server) GetBool(db uint16, off int, bit uint) (v bool, err error) {
	err = s.with(db, off, 1, func(b []byte) { v = b[0]&(1<<bit) != 0 })
	return
}

func (s *Server) SetInt(db uint16, off int, v int16) error {
	return s.with(db, off, 2, func(b []byte) { binary.BigEndian.PutUint16(b, uint16(v)) })
}

func (s *Server) GetInt(db uint16, off int) (v int16, err error) {
	err = s.with(db, off, 2, func(b []byte) { v = int16(binary.BigEndian.Uint16(b)) })
	return
}

func (s *Server) SetDInt(db uint16, off int, v int32) error {
	return s.with(db, off, 4, func(b []byte) { binary.BigEndian.PutUint32(b, uint32(v)) })
}

func (s *Server) GetDInt(db uint16, off int) (v int32, err error) {
	err = s.with(db, off, 4, func(b []byte) { v = int32(binary.BigEndian.Uint32(b)) })
	return
}

func (s *Server) SetReal(db uint16, off int, v float32) error {
	return s.with(db, off, 4, func(b []byte) { binary.BigEndian.PutUint32(b, math.Float32bits(v)) })
}

func (s *Server) GetReal(db uint16, off int) (v float32, err error) {
	err = s.with(db, off, 4, func(b []byte) { v = math.Float32frombits(binary.BigEndian.Uint32(b)) })
	return
}

// ---- Rede ----

// Start abre o listener e atende conexões em background.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return nil
}

// Stop encerra o listener.
func (s *Server) Stop() {
	if s.ln != nil {
		s.ln.Close()
	}
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	log.Printf("[S7 %s] cliente conectado: %s", s.Addr, c.RemoteAddr())
	defer log.Printf("[S7 %s] cliente desconectado: %s", s.Addr, c.RemoteAddr())
	for {
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(c, hdr); err != nil {
			return
		}
		if hdr[0] != 3 {
			return
		}
		total := int(binary.BigEndian.Uint16(hdr[2:4]))
		if total < 7 {
			return
		}
		payload := make([]byte, total-4)
		if _, err := io.ReadFull(c, payload); err != nil {
			return
		}
		var resp []byte
		switch payload[1] {
		case 0xE0: // COTP Connection Request -> Connection Confirm
			cc := append([]byte{}, payload...)
			cc[1] = 0xD0
			copy(cc[2:4], payload[4:6]) // dst ref = src ref do cliente
			cc[4], cc[5] = 0x00, 0x01   // src ref nosso
			resp = tpkt(cc)
		case 0xF0: // COTP Data
			li := int(payload[0])
			if len(payload) < li+1 {
				return
			}
			s7 := s.handleS7(payload[li+1:])
			if s7 == nil {
				continue
			}
			resp = tpkt(append([]byte{0x02, 0xF0, 0x80}, s7...))
		default:
			return
		}
		if _, err := c.Write(resp); err != nil {
			return
		}
	}
}

func tpkt(p []byte) []byte {
	out := []byte{3, 0, 0, 0}
	binary.BigEndian.PutUint16(out[2:], uint16(len(p)+4))
	return append(out, p...)
}

func ackHeader(pduRef []byte, plen, dlen int, errClass, errCode byte) []byte {
	h := []byte{0x32, 0x03, 0, 0, pduRef[0], pduRef[1], 0, 0, 0, 0, errClass, errCode}
	binary.BigEndian.PutUint16(h[6:], uint16(plen))
	binary.BigEndian.PutUint16(h[8:], uint16(dlen))
	return h
}

func (s *Server) handleS7(b []byte) []byte {
	if len(b) < 10 || b[0] != 0x32 || b[1] != 0x01 {
		return nil
	}
	pduRef := b[4:6]
	plen := int(binary.BigEndian.Uint16(b[6:8]))
	dlen := int(binary.BigEndian.Uint16(b[8:10]))
	if len(b) < 10+plen+dlen || plen < 1 {
		return nil
	}
	params := b[10 : 10+plen]
	data := b[10+plen : 10+plen+dlen]
	switch params[0] {
	case 0xF0: // Setup communication
		p := []byte{0xF0, 0, 0, 1, 0, 1, byte(maxPDU >> 8), byte(maxPDU & 0xFF)}
		return append(ackHeader(pduRef, len(p), 0, 0, 0), p...)
	case 0x04:
		return s.read(pduRef, params)
	case 0x05:
		return s.write(pduRef, params, data)
	}
	// função não suportada
	return ackHeader(pduRef, 0, 0, 0x81, 0x04)
}

type item struct {
	transport byte
	count     int
	db        uint16
	area      byte
	addr      int // endereço em bits
}

func parseItems(params []byte) []item {
	if len(params) < 2 {
		return nil
	}
	n := int(params[1])
	var items []item
	for i := 0; i < n; i++ {
		p := 2 + i*12
		if p+12 > len(params) || params[p] != 0x12 {
			return items
		}
		items = append(items, item{
			transport: params[p+3],
			count:     int(binary.BigEndian.Uint16(params[p+4:])),
			db:        binary.BigEndian.Uint16(params[p+6:]),
			area:      params[p+8],
			addr:      int(params[p+9])<<16 | int(params[p+10])<<8 | int(params[p+11]),
		})
	}
	return items
}

func elemSize(t byte) int {
	switch t {
	case 0x04, 0x05, 0x1C, 0x1D:
		return 2
	case 0x06, 0x07:
		return 4
	}
	return 1
}

func (s *Server) read(pduRef, params []byte) []byte {
	items := parseItems(params)
	var data []byte
	for i, it := range items {
		s.mu.RLock()
		mem, ok := s.areas[areaKey{it.area, it.db}]
		if it.area != AreaDB {
			mem, ok = s.areas[areaKey{it.area, 0}]
		}
		var out []byte
		var ts byte
		var bits int
		if ok {
			off := it.addr >> 3
			if it.transport == 0x01 {
				if off < len(mem) {
					out = []byte{(mem[off] >> uint(it.addr&7)) & 1}
					ts, bits = 0x03, 1
				}
			} else if n := it.count * elemSize(it.transport); off+n <= len(mem) {
				out = append([]byte{}, mem[off:off+n]...)
				ts, bits = 0x04, n*8
			}
		}
		s.mu.RUnlock()
		if out == nil {
			data = append(data, 0x05, 0x00, 0x00, 0x00) // endereço inválido
			continue
		}
		data = append(data, 0xFF, ts, byte(bits>>8), byte(bits))
		data = append(data, out...)
		if len(out)%2 == 1 && i < len(items)-1 {
			data = append(data, 0)
		}
	}
	p := []byte{0x04, byte(len(items))}
	return append(append(ackHeader(pduRef, len(p), len(data), 0, 0), p...), data...)
}

func (s *Server) write(pduRef, params, data []byte) []byte {
	items := parseItems(params)
	var results []byte
	d := 0
	for i, it := range items {
		if d+4 > len(data) {
			results = append(results, 0x0A)
			continue
		}
		ts := data[d+1]
		l := int(binary.BigEndian.Uint16(data[d+2:]))
		var n int
		switch ts {
		case 0x03:
			n = (l + 7) / 8
		case 0x04, 0x05:
			n = l / 8
		default:
			n = l
		}
		if d+4+n > len(data) {
			results = append(results, 0x0A)
			break
		}
		val := data[d+4 : d+4+n]
		d += 4 + n
		if n%2 == 1 && i < len(items)-1 {
			d++
		}
		s.mu.Lock()
		mem, ok := s.areas[areaKey{it.area, it.db}]
		if it.area != AreaDB {
			mem, ok = s.areas[areaKey{it.area, 0}]
		}
		rc := byte(0x05)
		off := it.addr >> 3
		if ok && off < len(mem) {
			if it.transport == 0x01 {
				bit := uint(it.addr & 7)
				if val[0]&1 == 1 {
					mem[off] |= 1 << bit
				} else {
					mem[off] &^= 1 << bit
				}
				rc = 0xFF
			} else if off+n <= len(mem) {
				copy(mem[off:], val)
				rc = 0xFF
			}
		}
		s.mu.Unlock()
		results = append(results, rc)
	}
	p := []byte{0x05, byte(len(items))}
	return append(append(ackHeader(pduRef, len(p), len(results), 0, 0), p...), results...)
}
