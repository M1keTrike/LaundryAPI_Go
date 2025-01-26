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
	name        string
	waterLevel  int
	energyLevel int
	mu          sync.Mutex
	busy        bool
}

const (
	MaxWaterPerWasher    = 80
	MaxEnergyPerWasher   = 80
	WaterLoadType1       = 10
	WaterLoadType2       = 20
	WaterLoadType3       = 30
	EnergyLoadType       = 30
	CycleDuration        = 3 * time.Second
	TankServerSupply     = "http://localhost:4006/supply?quantity=" // URL del tanque para suministro
	EnergyServerSupply   = "http://localhost:4008/supply?quantity=" // URL del proveedor de energía
)

var washers = []*Washer{
	{name: "washer1", waterLevel: MaxWaterPerWasher, energyLevel: MaxEnergyPerWasher},
	{name: "washer2", waterLevel: MaxWaterPerWasher, energyLevel: MaxEnergyPerWasher},
	{name: "washer3", waterLevel: MaxWaterPerWasher, energyLevel: MaxEnergyPerWasher},
}

func (w *Washer) useResources(waterAmount, energyAmount int) error {
	w.mu.Lock()
	if w.waterLevel < MaxWaterPerWasher {
		neededWater := MaxWaterPerWasher - w.waterLevel
		go refillWater(neededWater, w) // Reabastecimiento constante de agua
	}
	w.mu.Unlock()

	if w.energyLevel < energyAmount {
		neededEnergy := MaxEnergyPerWasher - w.energyLevel
		if err := refillEnergyAndDelegate(neededEnergy, w, waterAmount, energyAmount); err != nil {
			return err
		}
	}

	w.mu.Lock()
	if w.waterLevel >= waterAmount && w.energyLevel >= energyAmount {
		w.waterLevel -= waterAmount
		w.energyLevel -= energyAmount
		fmt.Printf("%s utilizó %d unidades de agua y %d unidades de energía. Nivel restante de agua: %d, energía: %d\n", w.name, waterAmount, energyAmount, w.waterLevel, w.energyLevel)
	} else {
		fmt.Printf("%s no tiene suficientes recursos para completar el ciclo.\n", w.name)
	}
	w.mu.Unlock()
	return nil
}

func refillWater(amount int, w *Washer) {
	resp, err := http.Post(TankServerSupply+strconv.Itoa(amount/10), "application/json", nil)
	if err != nil {
		fmt.Printf("%s no pudo obtener agua del tanque: %v\n", w.name, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("%s recibió un estado inesperado: %d\n", w.name, resp.StatusCode)
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			fmt.Printf("%s encontró un error al leer el agua: %v\n", w.name, err)
			return
		}

		var payload map[string]int
		if err := json.Unmarshal(line, &payload); err != nil {
			fmt.Printf("%s no pudo procesar el bloque de agua: %v\n", w.name, err)
			return
		}

		waterReceived := payload["water"]
		w.mu.Lock()
		w.waterLevel += waterReceived
		if w.waterLevel > MaxWaterPerWasher {
			w.waterLevel = MaxWaterPerWasher
		}
		fmt.Printf("%s recibió %d unidades de agua. Nivel actual: %d\n", w.name, waterReceived, w.waterLevel)
		w.mu.Unlock()
	}
}

func refillEnergyAndDelegate(amount int, w *Washer, waterAmount, energyAmount int) error {
	w.mu.Lock()
	w.busy = false
	w.mu.Unlock()

	fmt.Printf("%s está recargando energía y delegando la carga...\n", w.name)
	resp, err := http.Get(EnergyServerSupply + strconv.Itoa(amount))
	if err != nil {
		return fmt.Errorf("%s no pudo obtener energía del proveedor: %v", w.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s recibió un estado inesperado: %d", w.name, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("%s encontró un error al leer la energía: %v", w.name, err)
		}

		var payload map[string]int
		if err := json.Unmarshal(line, &payload); err != nil {
			return fmt.Errorf("%s no pudo procesar el bloque de energía: %v", w.name, err)
		}

		energyReceived := payload["energy"]
		w.mu.Lock()
		w.energyLevel += energyReceived
		if w.energyLevel > MaxEnergyPerWasher {
			w.energyLevel = MaxEnergyPerWasher
		}
		fmt.Printf("%s recibió %d unidades de energía. Nivel actual: %d\n", w.name, energyReceived, w.energyLevel)
		w.mu.Unlock()
	}

	// Delegar a otra lavadora si es necesario
	for _, otherWasher := range washers {
		if otherWasher != w {
			otherWasher.mu.Lock()
			if !otherWasher.busy {
				otherWasher.busy = true
				otherWasher.mu.Unlock()
				fmt.Printf("Delegando lavado a %s\n", otherWasher.name)
				go func() {
					_ = otherWasher.useResources(waterAmount, energyAmount)
				}()
				return nil
			}
			otherWasher.mu.Unlock()
		}
	}

	return nil
}

func manageWashing(loadType int, washer *Washer, c *gin.Context, done chan string) {
	var waterNeeded int
	var energyNeeded int = EnergyLoadType

	switch loadType {
	case 1:
		waterNeeded = WaterLoadType1
	case 2:
		waterNeeded = WaterLoadType2
	case 3:
		waterNeeded = WaterLoadType3
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s recibió una carga inválida", washer.name)})
		return
	}

	if err := washer.useResources(waterNeeded, energyNeeded); err != nil {
		fmt.Printf("%s no puede completar el lavado. Delegando a otra lavadora.\n", washer.name)
		for _, w := range washers {
			if w != washer {
				w.mu.Lock()
				if !w.busy {
					w.busy = true
					w.mu.Unlock()
					go manageWashing(loadType, w, c, done)
					return
				}
				w.mu.Unlock()
			}
		}
		c.JSON(http.StatusConflict, gin.H{"error": "No hay lavadoras disponibles para completar el lavado"})
		return
	}

	washer.mu.Lock()
	washer.busy = true
	washer.mu.Unlock()

	fmt.Printf("%s comenzó el ciclo de lavado con carga tipo %d\n", washer.name, loadType)
	time.Sleep(CycleDuration)
	fmt.Printf("%s terminó el ciclo de lavado\n", washer.name)

	washer.mu.Lock()
	washer.busy = false
	washer.mu.Unlock()

done <- fmt.Sprintf("Lavadora %s completó el ciclo de lavado con carga tipo %d", washer.name, loadType)
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

		var selectedWasher *Washer
		for _, washer := range washers {
			washer.mu.Lock()
			if !washer.busy {
				washer.busy = true
				selectedWasher = washer
				washer.mu.Unlock()
				break
			}
			washer.mu.Unlock()
		}

		if selectedWasher == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "No hay lavadoras disponibles"})
			return
		}

		// Crear un canal para notificar el fin del lavado
		done := make(chan string)

		// Iniciar el lavado en una gorutina
		go func() {
			manageWashing(loadType, selectedWasher, c, done)
		}()

		// Esperar a que se complete el ciclo de lavado
		result := <-done

		// Enviar respuesta HTTP con los detalles
		c.JSON(http.StatusOK, gin.H{
			"message": result,
			"details": gin.H{
				"load_type": loadType,
				"washer":    selectedWasher.name,
			},
		})
	})

	r.Run(":4007")
}
