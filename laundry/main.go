package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Washer struct {
	name       string
	waterLevel int
	mu         sync.Mutex
}

const (
	MaxWaterPerWasher = 80
	WaterLoadType1    = 10
	WaterLoadType2    = 20
	WaterLoadType3    = 30
	CycleDuration     = 3 * time.Second
	TankServerSupply  = "http://localhost:4006/supply?quantity=" // URL del tanque para suministro
	TankServerFill    = "http://localhost:4006/fill?quantity="   // URL del tanque para llenado
)

var washers = []*Washer{
	{name: "washer1", waterLevel: MaxWaterPerWasher},
	{name: "washer2", waterLevel: MaxWaterPerWasher},
	{name: "washer3", waterLevel: MaxWaterPerWasher},
}

func (w *Washer) useWater(amount int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.waterLevel < amount {
		neededWater := amount - w.waterLevel
		if err := requestWaterFromTank(neededWater, w); err != nil {
			return err
		}
	}

	w.waterLevel -= amount
	return nil
}

func requestWaterFromTank(amount int, w *Washer) error {
	resp, err := http.Get(TankServerSupply + strconv.Itoa(amount))
	if err != nil {
		return fmt.Errorf("%s no pudo obtener agua del tanque: %v", w.name, err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("%s encontró un error al leer el agua: %v", w.name, err)
		}

		var payload map[string]int
		if err := json.Unmarshal(line, &payload); err != nil {
			return fmt.Errorf("%s no pudo procesar el bloque de agua: %v", w.name, err)
		}

		waterReceived := payload["water"]
		w.mu.Lock()
		w.waterLevel += waterReceived
		w.mu.Unlock()
		fmt.Printf("%s recibió %d unidades de agua. Nivel actual: %d\n", w.name, waterReceived, w.waterLevel)
	}

	if resp.StatusCode == http.StatusConflict {
		fmt.Println("El tanque está vacío, llenándolo hasta la mitad...")
		_, fillErr := http.Post(TankServerFill+"75", "application/json", nil)
		if fillErr != nil {
			return fmt.Errorf("Error llenando el tanque: %v", fillErr)
		}
		time.Sleep(2 * time.Second) // Esperar un momento para que el tanque se recargue
	}

	return nil
}

func startWashing(loadType int, washer *Washer) {
	var waterNeeded int

	switch loadType {
	case 1:
		waterNeeded = WaterLoadType1
	case 2:
		waterNeeded = WaterLoadType2
	case 3:
		waterNeeded = WaterLoadType3
	default:
		fmt.Printf("%s recibió una carga inválida\n", washer.name)
		return
	}

	if err := washer.useWater(waterNeeded); err != nil {
		fmt.Printf("Error en %s: %v\n", washer.name, err)
		return
	}

	fmt.Printf("%s comenzó el ciclo de lavado con carga tipo %d\n", washer.name, loadType)
	time.Sleep(CycleDuration)
	fmt.Printf("%s terminó el ciclo de lavado\n", washer.name)
}

func main() {
	r := gin.Default()

	r.GET("/start", func(c *gin.Context) {
		loadTypeStr := c.Query("load")
		if loadTypeStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'load' es requerido"})
			return
		}

		loadType, err := strconv.Atoi(loadTypeStr)
		if err != nil || loadType < 1 || loadType > 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'load' debe ser 1, 2 o 3"})
			return
		}

		// Asignar lavadora disponible
		var selectedWasher *Washer
		for _, washer := range washers {
			if washer != nil {
				selectedWasher = washer
				break
			}
		}

		if selectedWasher == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No hay lavadoras disponibles"})
			return
		}

		go startWashing(loadType, selectedWasher)

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("%s comenzó el ciclo de lavado", selectedWasher.name),
		})
	})

	// Ejecutar el servidor en el puerto 4007
	r.Run(":4007")
}
