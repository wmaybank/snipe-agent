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
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Estructura para la configuración
type Config struct {
	SnipeHost string `yaml:"snipe_host"`
	SnipeKey  string `yaml:"snipe_key"`
	ModelID   int    `yaml:"model_id"`
	StatusID  int    `yaml:"status_id"`
}

// Estructura para los activos (añade más campos si es necesario)
type Asset struct {
	Name        string `json:"name"`
	Serial      string `json:"serial"`
	ModelID     int    `json:"model_id"`
	StatusID  int    `json:"status_id"`
	Category    string `json:"category"`
	Model       string `json:"model"`
	OSVersion   string `json:"os_version"`
	Hostname    string `json:"hostname"`
	CPU         string `json:"cpu"`
	RAM         string `json:"ram"`
	Storage     string `json:"storage"`
}

var config Config

func loadConfig() {
	configFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error al leer el archivo de configuración: %v", err)
	}

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Error al analizar la configuración: %v", err)
	}
}

func getOSVersion() string {
	return formatCmdOutput(runCommand("cmd", "/C", "ver")) // Comando simplificado
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		log.Println("ERROR: No se pudo obtener el hostname:", err)
		return "Desconocido" // Valor predeterminado
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
	// Obtener la memoria física total en KB, convertir a GB
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
	// Obtener tamaños de disco. Este es un ejemplo básico; puedes querer más detalles.
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
	// Intentar diferentes comandos para obtener el número de serie (más robusto)
	serial := formatWmicOutput(runWmicCommand("bios", "get", "serialnumber"))
	if serial == "" {
		serial = formatWmicOutput(runWmicCommand("csproduct", "get", "identifyingnumber"))
	}
	if serial == "" {
		serial = "Desconocido" // Valor predeterminado si no se encuentra
	}
	return serial
}

// ... (resto de las funciones: runWmicCommand, runCommand, formatWmicOutput, formatCmdOutput, apiPost)

func main() {
	loadConfig()

	newAsset := Asset{
		Name:        fmt.Sprintf("%s - %s", getHostname(), getCPUInfo()),
		Serial:      getSerialNumber(),
		ModelID:     config.ModelID,
		StatusID:    config.StatusID,
		Category:    "Desktop",
		Model:       getModelInfo(),
		OSVersion:   getOSVersion(),
		Hostname:    getHostname(),
		CPU         getCPUInfo(),
		RAM         getRAMSize(),
		Storage     getStorageInfo(),
	}

	err := createAsset(newAsset)
	if err != nil {
		log.Fatalf("Error al crear el activo: %v", err)
	}
}
