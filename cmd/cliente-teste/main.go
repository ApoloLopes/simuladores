// Cliente de teste: lê as tags dos dois simuladores usando bibliotecas
// de terceiros (gos7 para Siemens, gologix para Rockwell).
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/danomagnum/gologix"
	"github.com/robinson/gos7"
)

func main() {
	s7ip, enip := "127.0.0.1", "127.0.0.2"
	if len(os.Args) > 2 {
		s7ip, enip = os.Args[1], os.Args[2]
	}

	// ---- Siemens ----
	h := gos7.NewTCPClientHandler(s7ip, 0, 1)
	if err := h.Connect(); err != nil {
		log.Fatalf("S7 connect: %v", err)
	}
	defer h.Close()
	cl := gos7.NewClient(h)
	buf := make([]byte, 24)
	if err := cl.AGReadDB(1, 0, 24, buf); err != nil {
		log.Fatalf("S7 read: %v", err)
	}
	var hl gos7.Helper
	var v1 int16
	var t float32
	var dint int32
	hl.GetValueAt(buf, 2, &v1)
	hl.GetValueAt(buf, 4, &t)
	hl.GetValueAt(buf, 20, &dint)
	fmt.Printf("S7  Motor_Ligado=%v Velocidade(INT)=%d Temp(REAL)=%.2f Contador(DINT)=%d\n", buf[0]&1 == 1, v1, t, dint)

	// ---- Rockwell ----
	c := gologix.NewClient(enip)
	if err := c.Connect(); err != nil {
		log.Fatalf("ENIP connect: %v", err)
	}
	defer c.Disconnect()
	var temp, pres float32
	var st int16
	var cnt int32
	var mot bool
	for name, dst := range map[string]any{"Temperatura_Forno": &temp, "Pressao_Linha": &pres, "Status_Motor": &st, "Contador_Pecas": &cnt, "Motor_Ligado": &mot} {
		if err := c.Read(name, dst); err != nil {
			log.Fatalf("ENIP read %s: %v", name, err)
		}
	}
	fmt.Printf("ENIP Temp=%.2f Pressao=%.2f Status(INT)=%d Contador(DINT)=%d Motor(BOOL)=%v\n", temp, pres, st, cnt, mot)
	if err := c.Write("Status_Motor", int16(42)); err != nil {
		log.Fatalf("ENIP write: %v", err)
	}
	c.Read("Status_Motor", &st)
	fmt.Println("ENIP escrita Status_Motor ->", st)
}
