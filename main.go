// nim-cascade: proxy local que baja de modelo si NVIDIA responde con error.
// OpenCode -> nim-cascade (:8001) -> nim-proxy (:8000) -> NVIDIA NIM.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// De mejor a peor. Se puede cambiar con NIM_CASCADE_MODELS (separados por comas).
var defaultModels = []string{
	"nvidia/nemotron-3-ultra-550b-a55b",
	"moonshotai/kimi-k3.1-pro",
	"mistralai/mistral-large-4-1t-a52b",
	"z-ai/glm-5.3-flash",
	"minimaxai/minimax-m3.1-flash",
	"poolside/laguna-xs-2.1",
	"nvidia/nemotron-3.5-lightning-30b-a3b",
}

type cascade struct {
	upstream *url.URL
	models   []string
	rp       *httputil.ReverseProxy
}

func newCascade(upstream string, models []string) *cascade {
	u, err := url.Parse(upstream)
	if err != nil {
		log.Fatal(err)
	}
	return &cascade{upstream: u, models: models, rp: httputil.NewSingleHostReverseProxy(u)}
}

// Errores tras los que tiene sentido probar otro modelo.
func retryable(code int) bool { return code == 429 || code == 404 || code == 410 || code >= 500 }

func (c *cascade) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		c.rp.ServeHTTP(w, r)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cuerpo ilegible", http.StatusBadRequest)
		return
	}
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}
	// Empieza por el modelo pedido y baja desde ahí.
	start := 0
	asked, _ := body["model"].(string)
	for i, m := range c.models {
		if m == asked {
			start = i
		}
	}
	for i := start; i < len(c.models); i++ {
		last := i == len(c.models)-1
		body["model"] = c.models[i]
		b, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, c.upstream.String()+r.URL.Path, bytes.NewReader(b))
		req.Header = r.Header.Clone()
		req.Header.Del("Content-Length")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("%s: error de red: %v", c.models[i], err)
			if last {
				http.Error(w, "sin respuesta del upstream", http.StatusBadGateway)
				return
			}
			continue
		}
		if retryable(resp.StatusCode) && !last {
			log.Printf("%s: HTTP %d, bajo al siguiente modelo", c.models[i], resp.StatusCode)
			resp.Body.Close()
			continue
		}
		stream(w, resp)
		return
	}
}

// Copia la respuesta sin esperar al final, para que el streaming funcione.
func stream(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	f, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if f != nil {
				f.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	models := defaultModels
	if v := os.Getenv("NIM_CASCADE_MODELS"); v != "" {
		models = strings.Split(v, ",")
	}
	addr := env("NIM_CASCADE_ADDR", "127.0.0.1:8001")
	c := newCascade(env("NIM_UPSTREAM", "http://127.0.0.1:8000"), models)
	log.Printf("nim-cascade en %s -> %s (%d modelos)", addr, c.upstream, len(models))
	log.Fatal(http.ListenAndServe(addr, c))
}
