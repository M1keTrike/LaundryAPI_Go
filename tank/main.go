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

type Tank struct {
	capacity int16
	mutex    sync.Mutex
}

const (
	MAX_CAPACITY    int16 = 1500
	REFILL_THRESHOLD int16 = 1490
	WATER_SERVER_URL       = "http://localhost:4005/water?quantity=" // URL base del servidor de agua
	REFILL_QUANTITY        = 30
)

// Método para añadir agua al tanque
func (t *Tank) AddWater(amount int16) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.capacity+amount > MAX_CAPACITY {
		return false // No se puede añadir más agua porque supera la capacidad
	}
	t.capacity += amount
	fmt.Printf("Se añadieron %d unidades de agua al tanque. Capacidad actual: %d\n", amount, t.capacity)
	return true
}

// Método para obtener el estado del tanque
func (t *Tank) GetCapacity() int16 {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.capacity
}

// Método para usar agua del tanque
func (t *Tank) UseWater(amount int16) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.capacity < amount {
		return fmt.Errorf("no hay suficiente agua en el tanque")
	}
	t.capacity -= amount
	fmt.Printf("Se suministraron %d unidades de agua. Capacidad restante: %d\n", amount, t.capacity)
	return nil
}

// Función para gestionar el proceso de recarga
func (t *Tank) MonitorAndRefill() {
	for {
		t.mutex.Lock()
		if t.capacity < REFILL_THRESHOLD {
			fmt.Println("El nivel del tanque es bajo. Iniciando recarga...")
			t.mutex.Unlock()

			// Solicitar agua al servidor de SAPAM
			url := fmt.Sprintf("%s%d", WATER_SERVER_URL, REFILL_QUANTITY)
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("Error al solicitar recarga: %v\n", err)
				continue
			}
			defer resp.Body.Close()

			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err.Error() == "EOF" {
						break
					}
					fmt.Printf("Error leyendo respuesta: %v\n", err)
					break
				}

				var payload map[string]int
				if err := json.Unmarshal(line, &payload); err != nil {
					fmt.Printf("Error procesando bloque: %v\n", err)
					break
				}

				amount := int16(payload["water"])
				if !t.AddWater(amount) {
					fmt.Println("El tanque ha alcanzado su capacidad máxima durante la recarga.")
					break
				}
			}
		} else {
			t.mutex.Unlock()
		}

		time.Sleep(1 * time.Second) // Revisar el nivel del tanque cada segundo
	}
}

// Función para entregar agua en formato chunked
func deliverWaterChunked(c *gin.Context, tank *Tank, quantity int) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)

	waterChan := make(chan string)

	go func() {
		for i := 0; i < quantity; i++ {
			tank.mutex.Lock()
			if tank.capacity < 10 {
				tank.mutex.Unlock()
				fmt.Println("El tanque no tiene suficiente agua para suministrar más bloques.")
				break
			}
			tank.capacity -= 10
			fmt.Printf("Suministrando 10 unidades de agua. Capacidad restante: %d\n", tank.capacity)
			tank.mutex.Unlock()

			// Crear el bloque de agua
			block := map[string]int{"water": 10}
			blockJSON, _ := json.Marshal(block)
			waterChan <- string(blockJSON) + "\n"

			// Simular 1 segundo por bloque
			time.Sleep(1 * time.Second)
		}
		close(waterChan)
	}()

	// Enviar los bloques chunked
	for block := range waterChan {
		_, err := c.Writer.Write([]byte(block))
		if err != nil {
			fmt.Println("Error enviando bloque de agua:", err)
			break
		}
		c.Writer.Flush()
	}
	fmt.Println("Suministro de agua completado o interrumpido.")
}

func main() {
	tank := &Tank{capacity: MAX_CAPACITY} // Inicializar el tanque con capacidad máxima
	go tank.MonitorAndRefill()            // Iniciar monitoreo del nivel del tanque

	r := gin.Default()

	r.GET("/status", func(c *gin.Context) {
		// Devuelve el estado actual del tanque
		c.JSON(http.StatusOK, gin.H{
			"capacity":    tank.GetCapacity(),
			"max_capacity": MAX_CAPACITY,
		})
	})

	r.POST("/fill", func(c *gin.Context) {
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

		// Solicitar agua al servidor de agua
		resp, err := http.Get(WATER_SERVER_URL + strconv.Itoa(quantity))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("No se pudo obtener agua: %v", err)})
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error leyendo respuesta: %v", err)})
				return
			}

			var payload map[string]int
			if err := json.Unmarshal(line, &payload); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error procesando bloque: %v", err)})
				return
			}

			amount := int16(payload["water"])
			if !tank.AddWater(amount) {
				c.JSON(http.StatusConflict, gin.H{"error": "El tanque ha alcanzado su capacidad máxima"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message":         "Tanque llenado exitosamente",
			"current_capacity": tank.GetCapacity(),
		})
	})

	r.POST("/supply", func(c *gin.Context) {
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

		fmt.Printf("Recibida solicitud de suministro de %d unidades de agua. Capacidad actual: %d\n", quantity, tank.GetCapacity())
		deliverWaterChunked(c, tank, quantity)
	})

	// Ejecutar el servidor en el puerto 4006
	r.Run(":4006")
}
