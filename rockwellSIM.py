import sys
from cpppo.server.enip import main

def iniciar_clp_simulado():
    ip_clp = "127.0.0.2"  # Endereço exclusivo para o Rockwell
    
    tags = [
        "Temperatura_Forno=REAL",
        "Pressao_Linha=REAL",
        "Status_Motor=INT"
    ]

    print(f"Iniciando Servidor EtherNet/IP no IP {ip_clp} (Porta 44818)...")

    # Adicionamos o argumento -a seguido do IP
    sys.argv = ["enip_server", "-a", ip_clp, "-v"] + tags
    
    try:
        main.main()
    except KeyboardInterrupt:
        print("\nServidor encerrado.")

if __name__ == "__main__":
    iniciar_clp_simulado()