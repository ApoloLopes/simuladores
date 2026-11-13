package main

import (
	"fmt"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// CONFIGURAÇÃO DAS INSTÂNCIAS - edite aqui os IPs/portas.
// Podem ser sobrescritos por variáveis de ambiente (ex.: no Docker):
//   S7_IP, S7_PORT, ENIP_IP, ENIP_PORT
//   SIMULADORES=s7,rockwell   (quais instâncias subir; padrão: todas)
// No Windows/Linux, qualquer 127.x.x.x é loopback e dispensa configuração extra.
// ---------------------------------------------------------------------------

type Instancia struct {
	Nome string
	IP   string
	Porta string
}

func (i Instancia) Addr() string { return i.IP + ":" + i.Porta }

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func configuracao() (s7 []Instancia, rockwell []Instancia) {
	s7 = []Instancia{
		{Nome: "Siemens-S7-1", IP: env("S7_IP", "127.0.0.1"), Porta: env("S7_PORT", "102")},
	}
	rockwell = []Instancia{
		{Nome: "Rockwell-1", IP: env("ENIP_IP", "127.0.0.2"), Porta: env("ENIP_PORT", "44818")},
	}
	return
}

func simuladorAtivo(nome string) bool {
	lista := os.Getenv("SIMULADORES")
	if lista == "" {
		return true
	}
	for _, x := range strings.Split(lista, ",") {
		if strings.TrimSpace(x) == nome {
			return true
		}
	}
	return false
}

func (i Instancia) String() string { return fmt.Sprintf("%s @ %s", i.Nome, i.Addr()) }
