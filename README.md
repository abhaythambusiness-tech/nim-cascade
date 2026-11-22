# nim-cascade

Mini-proxy local en Go que baja automáticamente de modelo cuando la API gratuita de NVIDIA NIM falla o te limita (`429`, `404`, `410`, `5xx`). Sin dependencias externas: un solo archivo, ~130 líneas.

```
OpenCode  ->  nim-cascade (:8001)  ->  nim-proxy (:8000)  ->  NVIDIA NIM
              baja de modelo           reparte claves y
              si hay error             respeta 40 RPM por clave
```

Empieza por el modelo que pide el cliente y, si falla, prueba el siguiente de la lista. El último modelo devuelve su respuesta tal cual, sea cual sea. Soporta streaming.

## Requisitos

- Linux con systemd (para arranque automático; opcional)
- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/engine/install/)
- Una clave gratis de NVIDIA: https://build.nvidia.com (empieza por `nvapi-`)
- [OpenCode](https://opencode.ai)

## Instalación paso a paso

### 1. Arrancar nim-proxy (reparte claves y limita a 40 RPM)

```bash
docker run -d --name nim-proxy --restart unless-stopped -p 127.0.0.1:8000:8000 -v nim-proxy-data:/data ghcr.io/miztertea/nim-proxy:latest
```

Abre http://localhost:8000/, sigue el asistente y pega tu clave `nvapi-...`. Al final te da una clave `npk_...`. Es la que usará OpenCode.

### 2. Compilar nim-cascade

```bash
git clone https://github.com/abhaythambusiness-tech/nim-cascade.git
cd nim-cascade
go test ./... && mkdir -p ~/.local/bin && go build -o ~/.local/bin/nim-cascade .
```

### 3. Arrancarlo solo al encender el PC (opcional)

```bash
mkdir -p ~/.config/systemd/user && cp nim-cascade.service ~/.config/systemd/user/
systemctl --user enable --now nim-cascade
sudo loginctl enable-linger $USER
```

Para ver los logs: `journalctl --user -u nim-cascade -f`. Sin systemd, lanza `~/.local/bin/nim-cascade` a mano.

### 4. Guardar la clave `npk_` en un archivo privado

```bash
mkdir -p ~/.config/opencode
printf '%s' 'npk_tu_clave_aqui' > ~/.config/opencode/nim-proxy.key
chmod 600 ~/.config/opencode/nim-proxy.key
```

### 5. Configurar OpenCode

Crea `opencode.json` en tu proyecto (o `~/.config/opencode/opencode.json` para usarlo en todos):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "nim-proxy/nvidia/nemotron-3-ultra-550b-a55b",
  "provider": {
    "nim-proxy": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "NVIDIA NIM (nim-proxy local)",
      "options": {
        "baseURL": "http://localhost:8001/v1",
        "apiKey": "{file:~/.config/opencode/nim-proxy.key}"
      },
      "models": {
        "nvidia/nemotron-3-ultra-550b-a55b": { "name": "Nemotron 3 Ultra 550B" },
        "moonshotai/kimi-k3.1-pro": { "name": "Kimi K3.1 Pro" },
        "mistralai/mistral-large-4-1t-a52b": { "name": "Mistral Large 4" },
        "z-ai/glm-5.3-flash": { "name": "GLM 5.3 Flash" },
        "minimaxai/minimax-m3.1-flash": { "name": "MiniMax M3.1 Flash" },
        "poolside/laguna-xs-2.1": { "name": "Poolside Laguna XS 2.1" },
        "nvidia/nemotron-3.5-lightning-30b-a3b": { "name": "Nemotron 3.5 Lightning 30B" }
      }
    }
  }
}
```

Reinicia OpenCode, elige el modelo y escribe "hola".

## Configuración

Variables de entorno (en el servicio de systemd, añade líneas `Environment=...`):

| Variable | Por defecto | Qué hace |
|---|---|---|
| `NIM_CASCADE_ADDR` | `127.0.0.1:8001` | Dónde escucha |
| `NIM_UPSTREAM` | `http://127.0.0.1:8000` | Dirección de nim-proxy |
| `NIM_CASCADE_MODELS` | lista de `main.go` | Modelos separados por comas, de mejor a peor |

## Importante

- La lista de modelos cambia: NVIDIA retira modelos sin avisar (error `410 Gone`). Consulta los vigentes con `curl -s https://integrate.api.nvidia.com/v1/models` y ajusta `NIM_CASCADE_MODELS`.
- El orden de la lista por defecto es una estimación de calidad, no un benchmark.
- Solo escucha en `127.0.0.1`. No lo expongas a internet: reenvía la cabecera `Authorization` tal cual.
- Nunca subas tu `nvapi-...` ni tu `npk_...` a ningún repositorio.
- `nim-proxy` es un proyecto de terceros; revisa su código antes de darle tus claves.
