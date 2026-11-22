package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBajaDeModeloEnError(t *testing.T) {
	var vistos []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		m := b["model"].(string)
		vistos = append(vistos, m)
		if m == "a" {
			w.WriteHeader(429)
			return
		}
		w.Write([]byte("ok-" + m))
	}))
	defer up.Close()

	srv := httptest.NewServer(newCascade(up.URL, []string{"a", "b", "c"}))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"a"}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	if string(out) != "ok-b" || len(vistos) != 2 {
		t.Fatalf("respuesta %q, modelos probados %v", out, vistos)
	}
}
