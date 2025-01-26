package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	MaxEnergySupplyPerSecond = 10 // Máxima energía suministrada por segundo
)

type EnergyBlock struct {
	Energy int `json:"energy"`
}

func supplyEnergy(c *gin.Context) {
	quantityStr := c.Query("quantity")
	if quantityStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'quantity' es requerido"})
		return
	}

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'quantity' debe ser un número entero positivo"})
		return
	}

	// Calcular el número de bloques de energía a suministrar
	numBlocks := quantity / MaxEnergySupplyPerSecond
	if quantity%MaxEnergySupplyPerSecond != 0 {
		numBlocks++
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)

	for i := 0; i < numBlocks; i++ {
		energyToSupply := MaxEnergySupplyPerSecond
		if quantity < MaxEnergySupplyPerSecond {
			energyToSupply = quantity
		}
		quantity -= energyToSupply

		energyBlock := EnergyBlock{Energy: energyToSupply}
		blockJSON, _ := json.Marshal(energyBlock)
		_, err := c.Writer.Write([]byte(string(blockJSON) + "\n"))
		if err != nil {
			fmt.Println("Error enviando bloque de energía:", err)
			break
		}
		c.Writer.Flush()
		time.Sleep(1 * time.Second) // Simular envío de bloques de energía por segundo
	}
}

func main() {
	r := gin.Default()

	r.GET("/supply", supplyEnergy)

	// Ejecutar el servidor en el puerto 4008
	r.Run(":4008")
}
