import snap7
import time
import struct
from snap7.type import SrvArea

def main():
    # Instancia o servidor S7
    server = snap7.server.Server()
    
    # Cria um array de bytes de tamanho 100 para representar a DB 1
    db_size = 100
    db_data = bytearray(db_size)
    
    # ---------------------------------------------------------
    # Populando os dados na DB1 para o Go ler
    # ---------------------------------------------------------
    
    # 1. Motor_Ligado (BOOL) no Offset 0, Bit 0
    # Ligando o bit 0 do byte 0: 0000 0001
    db_data[0] = 0b00000001 
    
    # 2. Velocidade (INT) no Offset 2 (Ocupa 2 bytes)
    # '>h' = Big-Endian, short (2 bytes)
    velocidade = 1750
    struct.pack_into('>h', db_data, 2, velocidade)
    
    # 3. Temperatura (REAL) no Offset 4 (Ocupa 4 bytes)
    # '>f' = Big-Endian, float (4 bytes)
    temperatura = 45.8
    struct.pack_into('>f', db_data, 4, temperatura)

    temperatura_2 = 300.5

    struct.pack_into('>f', db_data, 8, temperatura_2)

    temperatura_3 = 300.5
    
    struct.pack_into('>f', db_data, 12, temperatura_3)

    temperatura_4 = 900.5
        
    struct.pack_into('>f', db_data, 16, temperatura_4)
    
    # Registra a área de memória como DB 1
    server.register_area(SrvArea.DB, 1, db_data)
    
    # Inicia o servidor (A porta padrão do S7 é a 102)
    # Nota: No Linux/Mac, abrir a porta 102 pode exigir rodar com sudo. No Windows costuma ser livre.
    server.start(102)
    print("Simulador S7 rodando na porta 102 (IP: 127.0.0.1)...")
    
    try:
        # Loop infinito para manter o servidor rodando e simular variação de dados
        while True:
            # Vamos simular a temperatura subindo aos poucos
            temperatura += 0.5
            struct.pack_into('>f', db_data, 4, temperatura)
            temperatura_2 -= 0.3
            struct.pack_into('>f', db_data, 8, temperatura_2)
            temperatura_3 += 0.2
            struct.pack_into('>f', db_data, 12, temperatura_3)
            temperatura_4 -= 0.1
            struct.pack_into('>f', db_data, 16, temperatura_4)
            # O Snap7 passa os dados por referência. Ao alterar db_data, a DB no servidor já atualiza.
            time.sleep(1)
            
    except KeyboardInterrupt:
        print("\nDesligando simulador...")
        server.stop()
        server.destroy()

if __name__ == "__main__":
    main()