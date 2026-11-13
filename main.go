// Simuladores de CLPs Siemens (S7comm) e Rockwell (EtherNet/IP) em Go.
package main

import (
	"log"
	"os"
	"os/signal"
	"time"

	"simuladores_clps/internal/enipsim"
	"simuladores_clps/internal/s7sim"
)

// Mapa da DB1 do Siemens (equivalente ao simuladorS7.py, com DINT e mais BOOLs).
//
//	DB1.DBX0.0  Motor_Ligado   BOOL
//	DB1.DBX0.1  Alarme         BOOL
//	DB1.DBW2    Velocidade     INT
//	DB1.DBD4    Temperatura_1  REAL
//	DB1.DBD8    Temperatura_2  REAL
//	DB1.DBD12   Temperatura_3  REAL
//	DB1.DBD16   Temperatura_4  REAL
//	DB1.DBD20   Contador_Pecas DINT
const db = 1

func iniciarS7(inst Instancia) *s7sim.Server {
	srv := s7sim.New(inst.Addr())
	srv.RegisterArea(s7sim.AreaDB, db, 100)

	srv.SetBool(db, 0, 0, true)
	srv.SetBool(db, 0, 1, false)
	srv.SetInt(db, 2, 1750)
	srv.SetReal(db, 4, 45.8)
	srv.SetReal(db, 8, 300.5)
	srv.SetReal(db, 12, 300.5)
	srv.SetReal(db, 16, 900.5)
	srv.SetDInt(db, 20, 100000)

	if err := srv.Start(); err != nil {
		log.Fatalf("falha ao iniciar %s: %v", inst, err)
	}
	log.Printf("Simulador S7 rodando: %s (DB%d)", inst, db)

	go func() {
		t1, t2, t3, t4 := float32(45.8), float32(300.5), float32(300.5), float32(900.5)
		var contador int32 = 100000
		for tick := 0; ; tick++ {
			time.Sleep(time.Second)
			t1 += 0.5
			t2 -= 0.3
			t3 += 0.2
			t4 -= 0.1
			contador++
			srv.SetReal(db, 4, t1)
			srv.SetReal(db, 8, t2)
			srv.SetReal(db, 12, t3)
			srv.SetReal(db, 16, t4)
			srv.SetDInt(db, 20, contador)
			srv.SetInt(db, 2, int16(1750+(tick%10)))
			srv.SetBool(db, 0, 1, tick%10 >= 8) // Alarme pisca
		}
	}()
	return srv
}

func iniciarRockwell(inst Instancia) *enipsim.Server {
	srv := enipsim.New(inst.Addr())
	srv.AddTag("Temperatura_Forno", enipsim.TypeReal)
	srv.AddTag("Pressao_Linha", enipsim.TypeReal)
	srv.AddTag("Status_Motor", enipsim.TypeInt)
	srv.AddTag("Motor_Ligado", enipsim.TypeBool)
	srv.AddTag("Contador_Pecas", enipsim.TypeDInt)

	srv.SetReal("Temperatura_Forno", 180.0)
	srv.SetReal("Pressao_Linha", 5.5)
	srv.SetInt("Status_Motor", 1)
	srv.SetBool("Motor_Ligado", true)
	srv.SetDInt("Contador_Pecas", 5000)

	if err := srv.Start(); err != nil {
		log.Fatalf("falha ao iniciar %s: %v", inst, err)
	}
	log.Printf("Simulador Rockwell EtherNet/IP rodando: %s", inst)

	go func() {
		for tick := 0; ; tick++ {
			time.Sleep(time.Second)
			temp, _ := srv.GetReal("Temperatura_Forno")
			srv.SetReal("Temperatura_Forno", temp+0.25)
			pres, _ := srv.GetReal("Pressao_Linha")
			srv.SetReal("Pressao_Linha", pres+0.01)
			cnt, _ := srv.GetDInt("Contador_Pecas")
			srv.SetDInt("Contador_Pecas", cnt+1)
			srv.SetInt("Status_Motor", int16(1+tick%3))
			srv.SetBool("Motor_Ligado", tick%10 < 7)
		}
	}()
	return srv
}

func main() {
	s7, rockwell := configuracao()
	if simuladorAtivo("s7") {
		for _, i := range s7 {
			defer iniciarS7(i).Stop()
		}
	}
	if simuladorAtivo("rockwell") {
		for _, i := range rockwell {
			defer iniciarRockwell(i).Stop()
		}
	}
	log.Println("Ctrl+C para encerrar.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Println("Desligando simuladores...")
}
