// Package enipsim implementa um servidor EtherNet/IP + CIP (Rockwell
// ControlLogix/CompactLogix) em Go puro, com tags simbólicas BOOL/SINT/INT/DINT/REAL.
package enipsim

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

// Tipos de dado CIP.
const (
	TypeBool uint16 = 0xC1
	TypeSInt uint16 = 0xC2
	TypeInt  uint16 = 0xC3
	TypeDInt uint16 = 0xC4
	TypeReal uint16 = 0xCA
)

var typeSize = map[uint16]int{TypeBool: 1, TypeSInt: 1, TypeInt: 2, TypeDInt: 4, TypeReal: 4}

type tag struct {
	name string
	typ  uint16
	data []byte
}

// Server é um servidor EtherNet/IP simulado.
type Server struct {
	Addr string

	mu   sync.Mutex
	tags map[string]*tag

	connMu  sync.Mutex
	conns   map[uint32]uint32 // O->T id -> T->O id
	nextID  uint32
	session uint32
	ln      net.Listener
}

// New cria um servidor que escutará em addr ("ip:porta").
func New(addr string) *Server {
	return &Server{Addr: addr, tags: map[string]*tag{}, conns: map[uint32]uint32{}, nextID: 0x1000}
}

// AddTag cria uma tag com valor zero.
func (s *Server) AddTag(name string, typ uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags[strings.ToLower(name)] = &tag{name: name, typ: typ, data: make([]byte, typeSize[typ])}
}

func (s *Server) with(name string, fn func(t *tag)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tags[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("tag %q não existe", name)
	}
	fn(t)
	return nil
}

func (s *Server) SetBool(name string, v bool) error {
	return s.with(name, func(t *tag) {
		t.data[0] = 0
		if v {
			t.data[0] = 0xFF
		}
	})
}
func (s *Server) GetBool(name string) (v bool, err error) {
	err = s.with(name, func(t *tag) { v = t.data[0] != 0 })
	return
}
func (s *Server) SetInt(name string, v int16) error {
	return s.with(name, func(t *tag) { binary.LittleEndian.PutUint16(t.data, uint16(v)) })
}
func (s *Server) GetInt(name string) (v int16, err error) {
	err = s.with(name, func(t *tag) { v = int16(binary.LittleEndian.Uint16(t.data)) })
	return
}
func (s *Server) SetDInt(name string, v int32) error {
	return s.with(name, func(t *tag) { binary.LittleEndian.PutUint32(t.data, uint32(v)) })
}
func (s *Server) GetDInt(name string) (v int32, err error) {
	err = s.with(name, func(t *tag) { v = int32(binary.LittleEndian.Uint32(t.data)) })
	return
}
func (s *Server) SetReal(name string, v float32) error {
	return s.with(name, func(t *tag) { binary.LittleEndian.PutUint32(t.data, math.Float32bits(v)) })
}
func (s *Server) GetReal(name string) (v float32, err error) {
	err = s.with(name, func(t *tag) { v = math.Float32frombits(binary.LittleEndian.Uint32(t.data)) })
	return
}

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

var sessionCounter uint32

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	log.Printf("[ENIP %s] cliente conectado: %s", s.Addr, c.RemoteAddr())
	defer log.Printf("[ENIP %s] cliente desconectado: %s", s.Addr, c.RemoteAddr())
	for {
		h := make([]byte, 24)
		if _, err := io.ReadFull(c, h); err != nil {
			return
		}
		cmd := binary.LittleEndian.Uint16(h[0:])
		n := int(binary.LittleEndian.Uint16(h[2:]))
		data := make([]byte, n)
		if _, err := io.ReadFull(c, data); err != nil {
			return
		}
		var reply []byte
		switch cmd {
		case 0x65: // RegisterSession
			binary.LittleEndian.PutUint32(h[4:], atomic.AddUint32(&sessionCounter, 1))
			reply = data
		case 0x66: // UnRegisterSession
			return
		case 0x04: // ListServices
			reply = []byte{1, 0, 0x01, 0x01, 20, 0, 1, 0, 0x20, 0, 'C', 'o', 'm', 'm', 'u', 'n', 'i', 'c', 'a', 't', 'i', 'o', 'n', 's', 0, 0}
		case 0x64: // ListInterfaces
			reply = []byte{0, 0}
		case 0x6F: // SendRRData
			reply = s.sendRR(data)
		case 0x70: // SendUnitData
			reply = s.sendUnit(data)
			if reply == nil {
				continue
			}
		default:
			binary.LittleEndian.PutUint32(h[8:], 1) // comando inválido
			reply = nil
		}
		binary.LittleEndian.PutUint16(h[2:], uint16(len(reply)))
		if _, err := c.Write(append(h, reply...)); err != nil {
			return
		}
	}
}

type cpfItem struct {
	typ  uint16
	data []byte
}

func parseCPF(b []byte) []cpfItem {
	if len(b) < 2 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(b))
	p := 2
	var items []cpfItem
	for i := 0; i < n && p+4 <= len(b); i++ {
		t := binary.LittleEndian.Uint16(b[p:])
		l := int(binary.LittleEndian.Uint16(b[p+2:]))
		if p+4+l > len(b) {
			break
		}
		items = append(items, cpfItem{t, b[p+4 : p+4+l]})
		p += 4 + l
	}
	return items
}

func buildCPF(items ...cpfItem) []byte {
	out := []byte{0, 0, 0, 0, 0, 0} // interface handle + timeout
	out = binary.LittleEndian.AppendUint16(out, uint16(len(items)))
	for _, it := range items {
		out = binary.LittleEndian.AppendUint16(out, it.typ)
		out = binary.LittleEndian.AppendUint16(out, uint16(len(it.data)))
		out = append(out, it.data...)
	}
	return out
}

func (s *Server) sendRR(data []byte) []byte {
	if len(data) < 6 {
		return nil
	}
	items := parseCPF(data[6:])
	for _, it := range items {
		if it.typ == 0xB2 {
			return buildCPF(cpfItem{0, nil}, cpfItem{0xB2, s.process(it.data)})
		}
	}
	return nil
}

func (s *Server) sendUnit(data []byte) []byte {
	if len(data) < 6 {
		return nil
	}
	items := parseCPF(data[6:])
	if len(items) < 2 || items[0].typ != 0xA1 || items[1].typ != 0xB1 || len(items[1].data) < 2 {
		return nil
	}
	ot := binary.LittleEndian.Uint32(items[0].data)
	s.connMu.Lock()
	to, ok := s.conns[ot]
	s.connMu.Unlock()
	if !ok {
		return nil
	}
	cip := s.process(items[1].data[2:])
	addr := binary.LittleEndian.AppendUint32(nil, to)
	return buildCPF(cpfItem{0xA1, addr}, cpfItem{0xB1, append(append([]byte{}, items[1].data[:2]...), cip...)})
}

// reply monta a resposta CIP: serviço|0x80, reservado, status, extended size, dados.
func reply(svc byte, status byte, ext []byte, data []byte) []byte {
	out := []byte{svc | 0x80, 0, status, byte(len(ext) / 2)}
	out = append(out, ext...)
	return append(out, data...)
}

// process trata uma mensagem CIP e devolve a resposta.
func (s *Server) process(req []byte) []byte {
	if len(req) < 2 {
		return reply(0, 0x04, nil, nil)
	}
	svc := req[0]
	pathLen := int(req[1]) * 2
	if len(req) < 2+pathLen {
		return reply(svc, 0x04, nil, nil)
	}
	path := req[2 : 2+pathLen]
	body := req[2+pathLen:]

	switch svc {
	case 0x4C:
		return s.readTag(path, body)
	case 0x4D:
		return s.writeTag(path, body)
	case 0x52: // Unconnected Send
		if len(body) < 4 {
			return reply(svc, 0x13, nil, nil)
		}
		size := int(binary.LittleEndian.Uint16(body[2:]))
		if 4+size > len(body) {
			return reply(svc, 0x13, nil, nil)
		}
		return s.process(body[4 : 4+size])
	case 0x0A: // Multiple Service Packet
		return s.multi(body)
	case 0x54, 0x5B: // Forward Open / Large Forward Open
		return s.forwardOpen(svc, body)
	case 0x4E, 0x5C: // Forward Close
		if len(body) < 10 {
			return reply(svc, 0x13, nil, nil)
		}
		out := append([]byte{}, body[2:10]...) // serial, vendor, originator serial
		out = append(out, 0, 0)
		return reply(svc, 0, nil, out)
	}
	return reply(svc, 0x08, nil, nil) // serviço não suportado
}

func (s *Server) multi(body []byte) []byte {
	if len(body) < 2 {
		return reply(0x0A, 0x13, nil, nil)
	}
	n := int(binary.LittleEndian.Uint16(body))
	var replies [][]byte
	for i := 0; i < n; i++ {
		if 2+i*2+2 > len(body) {
			break
		}
		start := int(binary.LittleEndian.Uint16(body[2+i*2:]))
		end := len(body)
		if i+1 < n {
			end = int(binary.LittleEndian.Uint16(body[2+(i+1)*2:]))
		}
		if start > end || end > len(body) {
			break
		}
		replies = append(replies, s.process(body[start:end]))
	}
	out := binary.LittleEndian.AppendUint16(nil, uint16(len(replies)))
	off := 2 + 2*len(replies)
	for _, r := range replies {
		out = binary.LittleEndian.AppendUint16(out, uint16(off))
		off += len(r)
	}
	for _, r := range replies {
		out = append(out, r...)
	}
	return reply(0x0A, 0, nil, out)
}

func (s *Server) forwardOpen(svc byte, b []byte) []byte {
	large := svc == 0x5B
	min := 36
	if large {
		min = 40
	}
	if len(b) < min {
		return reply(svc, 0x13, nil, nil)
	}
	toID := binary.LittleEndian.Uint32(b[6:])
	s.connMu.Lock()
	s.nextID++
	otID := s.nextID
	s.conns[otID] = toID
	s.connMu.Unlock()

	otRPI := b[22:26]
	var toRPI []byte
	if large {
		toRPI = b[30:34]
	} else {
		toRPI = b[28:32]
	}
	out := binary.LittleEndian.AppendUint32(nil, otID)
	out = binary.LittleEndian.AppendUint32(out, toID)
	out = append(out, b[10:18]...) // serial, vendor, originator serial
	out = append(out, otRPI...)
	out = append(out, toRPI...)
	out = append(out, 0, 0)
	return reply(svc, 0, nil, out)
}

// parseTagPath extrai o nome simbólico de um caminho CIP (segmentos 0x91).
func parseTagPath(p []byte) (string, bool) {
	var parts []string
	for i := 0; i < len(p); {
		switch p[i] {
		case 0x91:
			if i+2 > len(p) {
				return "", false
			}
			l := int(p[i+1])
			if i+2+l > len(p) {
				return "", false
			}
			parts = append(parts, string(p[i+2:i+2+l]))
			i += 2 + l + l%2
		case 0x28:
			if p[i+1] != 0 {
				return "", false
			}
			i += 2
		case 0x29:
			if i+4 > len(p) || binary.LittleEndian.Uint16(p[i+2:]) != 0 {
				return "", false
			}
			i += 4
		default:
			return "", false
		}
	}
	return strings.Join(parts, "."), len(parts) > 0
}

func (s *Server) readTag(path, body []byte) []byte {
	name, ok := parseTagPath(path)
	if !ok || len(body) < 2 {
		return reply(0x4C, 0x04, nil, nil)
	}
	if binary.LittleEndian.Uint16(body) != 1 {
		return reply(0x4C, 0x05, nil, nil)
	}
	var out []byte
	if s.with(name, func(t *tag) {
		out = binary.LittleEndian.AppendUint16(nil, t.typ)
		out = append(out, t.data...)
	}) != nil {
		return reply(0x4C, 0x05, nil, nil) // destino desconhecido
	}
	return reply(0x4C, 0, nil, out)
}

func (s *Server) writeTag(path, body []byte) []byte {
	name, ok := parseTagPath(path)
	if !ok || len(body) < 4 {
		return reply(0x4D, 0x04, nil, nil)
	}
	typ := binary.LittleEndian.Uint16(body)
	if binary.LittleEndian.Uint16(body[2:]) != 1 {
		return reply(0x4D, 0x05, nil, nil)
	}
	val := body[4:]
	status := byte(0)
	var ext []byte
	if s.with(name, func(t *tag) {
		switch {
		case t.typ != typ:
			status, ext = 0xFF, []byte{0x07, 0x21}
		case len(val) < len(t.data):
			status = 0x13
		default:
			copy(t.data, val[:len(t.data)])
		}
	}) != nil {
		return reply(0x4D, 0x05, nil, nil)
	}
	return reply(0x4D, status, ext, nil)
}
