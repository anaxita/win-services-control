package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gorilla/mux"
)

type Config struct {
	Port          string   `json:"port"`
	AllowServices []string `json:"allow_services"`
}

//go:embed index.html
var indexHtml embed.FS

func main() {
	var cfg Config
	raw, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
		return
	}

	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatal(err)
	}

	log.Println("Config is:", cfg)

	r := mux.NewRouter()

	home := r.PathPrefix("/home").Subrouter()
	home.Use(cors)
	home.HandleFunc("", homeController())

	control := r.PathPrefix("/control").Subrouter()
	control.Use(cors)
	control.HandleFunc("", controlController(cfg.AllowServices))

	services := r.PathPrefix("/services").Subrouter()
	services.Use(cors)
	services.HandleFunc("", showServices(cfg.AllowServices)).Methods(http.MethodGet)

	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Add("Allow-Access-Control-Origin", "http://localhost")
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}
func homeController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		html, err := template.ParseFS(indexHtml, "index.html")
		// html, err := template.ParseFiles("index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		err = html.Execute(w, nil)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func controlController(allowServices []string) http.HandlerFunc {
	type Command struct {
		Action      string `json:"action"`
		ServiceName string `json:"service"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		command := Command{}

		if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !isAllowService(allowServices, command.ServiceName) {
			http.Error(w, "Service is not allowed", http.StatusForbidden)
			// log.Println("Service is now allowed")
			return
		}

		switch command.Action {
		case "stop":
			script := fmt.Sprintf("stop-Service -Name %s", command.ServiceName)
			cmd := exec.Command("powershell", "/C", script)
			err := cmd.Run()
			if err != nil {
				http.Error(w, "Service is not allowed", http.StatusForbidden)
				// log.Println(err)
				return
			}
			w.WriteHeader(http.StatusOK)

		case "start":
			script := fmt.Sprintf("Start-Service -Name %s", command.ServiceName)
			cmd := exec.Command("powershell", "/C", script)
			err := cmd.Run()
			if err != nil {
				http.Error(w, "Service is not allowed", http.StatusForbidden)
				// log.Println(err)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
}

func showServices(allowServices []string) http.HandlerFunc {
	type WinServices struct {
		Name        string `json:"name"`
		State       string `json:"state"`
		DisplayName string `json:"display_name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		servicesList := strings.Join(allowServices, ", ")
		script := fmt.Sprintf("[Console]::OutputEncoding = [System.Text.Encoding]::GetEncoding('utf-8'); Get-Service -Name %s | Select-Object @{n='name'; e={$_.Name}}, @{n='display_name'; e={$_.DisplayName}}, @{n='state'; e={[string]$_.Status}} | ConvertTo-Json", servicesList)
		// log.Println(script)
		cmd := exec.Command("powershell", "/C", script)
		out, err := cmd.Output()

		if err != nil {
			http.Error(w, "Service is not allowed", http.StatusForbidden)
			// log.Println(err)
			return
		}

		var services []WinServices
		var service WinServices

		err = json.Unmarshal(out, &services)
		err_ := json.Unmarshal(out, &service)

		if err != nil && err_ != nil {
			http.Error(w, "Error get services", http.StatusForbidden)
			// log.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
		if services != nil {
			err = json.NewEncoder(w).Encode(services)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				// log.Println(err)
			}
		} else {

			err = json.NewEncoder(w).Encode(append(services, service))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				// log.Println(err)
			}
		}
	}
}
func isAllowService(services []string, service string) bool {
	for _, v := range services {
		if v == service {
			return true
		}
	}

	return false
}
