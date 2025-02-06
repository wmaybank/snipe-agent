package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Estructura para la configuración
type Config struct {
	SnipeHost string `yaml:"snipe_host"`
	SnipeKey  string `yaml:"snipe_key"`
	StatusID  int    `yaml:"status_id"`
}

// Estructura para los activos
type Asset struct {
	Name       string `json:"name"`
	Serial     string `json:"serial"`
	ModelID    int    `json:"model_id"`
	StatusID   int    `json:"status_id"`
	CategoryID int    `json:"category_id"`
	Model      string `json:"model"`
	OSVersion  string `json:"os_version"`
	Hostname   string `json:"hostname"`
	CPU        string `json:"cpu"`
	RAM        string `json:"ram"`
	Storage    string `json:"storage"`
}

var config Config

// Cargar configuración desde config.yaml
func loadConfig() {
	configFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("❌ Error al leer el archivo de configuración: %v", err)
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("❌ Error al analizar la configuración: %v", err)
	}

	if config.SnipeHost == "" || config.SnipeKey == "" {
		log.Fatal("❌ ERROR: Faltan valores en config.yaml. Asegúrate de incluir 'snipe_host' y 'snipe_key'.")
	}

	fmt.Println("✅ Configuración cargada correctamente.")
	fmt.Println("🌐 API URL:", config.SnipeHost)
	fmt.Println("🔑 API Key:", config.SnipeKey[:5]+"********")
}

// Función para hacer solicitudes a la API de Snipe-IT
func apiPost(endpoint string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("❌ Error al serializar JSON: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("POST", config.SnipeHost+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("❌ Error al crear solicitud POST: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.SnipeKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	fmt.Println("📡 Enviando solicitud a:", config.SnipeHost+endpoint)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("❌ Error al realizar solicitud POST: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	fmt.Println("🔄 Código de estado HTTP:", resp.StatusCode)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Println("❌ Respuesta de la API:", string(bodyBytes))
		return nil, fmt.Errorf("ERROR: Código de estado %d", resp.StatusCode)
	}

	return bodyBytes, nil
}

// Crear activo en Snipe-IT
func createAsset(asset Asset) error {
	response, err := apiPost("/hardware", asset)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	json.Unmarshal(response, &result)

	// Mostrar detalles del activo creado
	log.Println("✅ Activo creado con éxito:")
	log.Println("📌 ID del activo:", result["payload"].(map[string]interface{})["id"])
	log.Println("🏷️  Nombre:", result["payload"].(map[string]interface{})["name"])
	log.Println("🔢 Serial:", result["payload"].(map[string]interface{})["serial"])
	log.Println("📦 Modelo:", result["payload"].(map[string]interface{})["model"].(map[string]interface{})["name"])

	return nil
}

// Función principal
func main() {
	loadConfig()

	newAsset := Asset{
		Name:       fmt.Sprintf("%s - %s", strings.TrimSpace(getHostname()), strings.TrimSpace(getCPUInfo())),
		Serial:     strings.TrimSpace(getSerialNumber()),
		ModelID:    2,
		StatusID:   config.StatusID,
		CategoryID: 2,
		Model:      strings.TrimSpace(getModelInfo()),
		OSVersion:  strings.TrimSpace(getOSVersion()),
		Hostname:   strings.TrimSpace(getHostname()),
		CPU:        strings.TrimSpace(getCPUInfo()),
		RAM:        strings.TrimSpace(getRAMSize()),
		Storage:    strings.TrimSpace(getStorageInfo()),
	}

	log.Printf("📊 Datos del activo: %+v", newAsset)

	err := createAsset(newAsset)
	if err != nil {
		log.Fatalf("❌ Error al crear el activo: %v", err)
	}
}

// Funciones para obtener información del sistema
func getOSVersion() string {
	return formatCmdOutput(runCommand("cmd", "/C", "ver"))
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		log.Println("❌ ERROR: No se pudo obtener el hostname:", err)
		return "Desconocido"
	}
	return strings.TrimSpace(name)
}

func getCPUInfo() string {
	return formatWmicOutput(runWmicCommand("cpu", "get", "name"))
}

func getModelInfo() string {
	return formatWmicOutput(runWmicCommand("computersystem", "get", "model"))
}

func getRAMSize() string {
	output := formatWmicOutput(runWmicCommand("os", "get", "TotalVisibleMemorySize"))
	if output == "" {
		return "Desconocido"
	}
	var memKB uint64
	fmt.Sscan(output, &memKB)
	memGB := float64(memKB) / (1024 * 1024)
	return fmt.Sprintf("%.2f GB", memGB)
}

func getStorageInfo() string {
	output := formatWmicOutput(runWmicCommand("diskdrive", "get", "size"))
	sizes := strings.Split(output, ",")
	var storageInfo []string
	for _, sizeStr := range sizes {
		var sizeBytes uint64
		fmt.Sscan(sizeStr, &sizeBytes)
		sizeGB := float64(sizeBytes) / (1024 * 1024 * 1024)
		storageInfo = append(storageInfo, fmt.Sprintf("%.2f GB", sizeGB))
	}
	return strings.Join(storageInfo, ", ")
}

func getSerialNumber() string {
	serial := formatWmicOutput(runWmicCommand("bios", "get", "serialnumber"))
	if serial == "" {
		serial = formatWmicOutput(runWmicCommand("csproduct", "get", "identifyingnumber"))
	}
	if serial == "" {
		serial = "Desconocido"
	}
	return serial
}

// Funciones para ejecutar comandos
func runWmicCommand(args ...string) string {
	cmd := exec.Command("wmic", args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("❌ ERROR: No se pudo ejecutar el comando wmic: %v", err)
		return "Error: " + err.Error()
	}
	return string(output)
}

func runCommand(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("❌ ERROR: No se pudo ejecutar el comando %s: %v\n", command, err)
		return "Error: " + err.Error()
	}
	return string(output)
}

func formatWmicOutput(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 1 {
		return strings.TrimSpace(strings.Join(lines[1:], ", "))
	}
	return strings.TrimSpace(output)
}

func formatCmdOutput(output string) string {
	return strings.TrimSpace(output)
}
