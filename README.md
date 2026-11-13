# Simuladores de CLPs Industriais em Go (Siemens S7 & Rockwell EtherNet/IP)

Este repositório contém simuladores de CLP desenvolvidos em **Go puro**, implementando diretamente os protocolos industriais:
- **Siemens S7comm** (ISO-on-TCP / TPKT / COTP / S7 PDU) - Porta `102`
- **Rockwell / Allen-Bradley EtherNet/IP** (CIP / Unconnected Send / Tag Addressing) - Porta `44818`

Compatível com execução local (Windows/Linux), containers Docker e sistemas embarcados ARM (ex: **Advantech ECU-1051**).

---

## 📋 Tags e Endereçamentos Disponíveis

### 1. Siemens S7 (Bloco de Dados DB1)
- **Protocolo:** S7comm (Porta 102)
- **Configuração de Rack/Slot:** Rack `0`, Slot `1` ou `2`
- **Endereço de Memória:** DB1 (100 bytes)

| Tag Simbólica | Endereço S7 | Tipo | Tamanho | Descrição |
| :--- | :--- | :--- | :--- | :--- |
| `Motor_Ligado` | `DB1.DBX0.0` | **BOOL** | 1 bit | Status Ligado (Fixo em `true`) |
| `Alarme` | `DB1.DBX0.1` | **BOOL** | 1 bit | Alarme pulsante (alterna ciclicamente) |
| `Velocidade` | `DB1.DBW2` | **INT** | 16 bits | Velocidade em RPM (~1750 a 1759) |
| `Temperatura_1`| `DB1.DBD4` | **REAL** | 32 bits | Inicia em 45.8 e varia +0.5/s |
| `Temperatura_2`| `DB1.DBD8` | **REAL** | 32 bits | Inicia em 300.5 e varia -0.3/s |
| `Temperatura_3`| `DB1.DBD12`| **REAL** | 32 bits | Inicia em 300.5 e varia +0.2/s |
| `Temperatura_4`| `DB1.DBD16`| **REAL** | 32 bits | Inicia em 900.5 e varia -0.1/s |
| `Contador_Pecas`| `DB1.DBD20`| **DINT** | 32 bits | Contador incremental (+1/s) |

---

### 2. Rockwell / Allen-Bradley (EtherNet/IP CIP)
- **Protocolo:** EtherNet/IP (Porta 44818)
- **Acesso:** Tag Simbólica (Symbolic Tag Name)

| Nome da Tag | Tipo CIP | Tamanho | Descrição / Comportamento |
| :--- | :--- | :--- | :--- |
| `Motor_Ligado` | **BOOL** | 1 byte | Alterna entre `true` e `false` |
| `Status_Motor` | **INT** | 2 bytes | Alterna entre 1, 2 e 3 (**Aceita escrita**) |
| `Contador_Pecas`| **DINT** | 4 bytes | Contador de produção incremental |
| `Temperatura_Forno`| **REAL** | 4 bytes | Simulação de temperatura (180.0 +0.25/s) |
| `Pressao_Linha` | **REAL** | 4 bytes | Simulação de pressão (5.5 +0.01/s) |

---

## 🚀 Como Executar

### 1. Localmente (Desenvolvimento / Windows / Linux)
Por padrão, sobe o Siemens em `127.0.0.1:102` e o Rockwell em `127.0.0.2:44818`:
```bash
go run .
```

Você pode customizar os IPs via variáveis de ambiente:
```bash
S7_IP=192.168.1.50 ENIP_IP=192.168.1.51 go run .
```

### 2. Executando via Docker Compose
Sobe as duas instâncias em containers isolados com IPs fixos:
```bash
docker compose up --build
```

### 3. Testando a Leitura/Escrita com o Cliente de Teste
O projeto inclui um cliente de teste para validação das tags:
```bash
go run ./cmd/cliente-teste [IP_SIEMENS] [IP_ROCKWELL]
# Exemplo:
go run ./cmd/cliente-teste 192.168.1.50 192.168.1.51
```

---

## 🛠️ Compilação e Deploy para Processadores ARM (ECU-1051)

Para compilar o binário estático para processadores ARMv7 (como o Advantech ECU-1051):

### No Windows (PowerShell):
```powershell
.\build-arm.ps1
```
O executável será gerado em: `dist/simuladores-linux-armv7`.

### Copiar para o ARM via SCP:
```powershell
scp dist\simuladores-linux-armv7 root@192.168.1.50:/home/simuladores/simuladores-linux-armv7
```

### Inicialização Automática no Boot (Yocto / BusyBox):
No terminal do ARM, adicione a chamada no `/etc/rc.local`:
```bash
mount -o remount,rw /
echo 'S7_IP=192.168.1.50 ENIP_IP=192.168.1.51 /home/simuladores/simuladores-linux-armv7 > /dev/null 2>&1 &' >> /etc/rc.local
chmod +x /etc/rc.local
mount -o remount,ro /
```

---

## ⚙️ Como Adicionar Novas Tags

Todas as tags e lógicas de atualização contínua estão no arquivo [`main.go`](main.go):

1. **Siemens S7:** Registre o valor inicial em `iniciarS7()` com `srv.SetBool()`, `srv.SetInt()`, `srv.SetDInt()` ou `srv.SetReal()`, informando o offset da DB.
2. **Rockwell:** Registre a tag em `iniciarRockwell()` com `srv.AddTag("Nome", enipsim.Type...)` e defina o valor inicial.
3. Para mais detalhes, consulte o arquivo de referência [`INFORMACOES_SIMULADORES.txt`](INFORMACOES_SIMULADORES.txt).
