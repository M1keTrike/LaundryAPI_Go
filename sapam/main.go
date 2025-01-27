package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Función que simula la generación de agua en bloques
func deliverWater(c *gin.Context, quantity int) {
	// Crear un canal para los bloques de agua
	waterChan := make(chan string)

	// Goroutine para generar agua
	go func() {
		for i := 0; i < quantity; i++ {
			// Esperar 1 segundo por bloque
			time.Sleep(1 * time.Second)

			// Enviar un bloque de agua al canal
			waterChan <- "{ \"water\": 10}\n"
		}
		close(waterChan) // Cerrar el canal al finalizar
	}()

	// Configurar la respuesta como chunked
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)

	// Transmitir los bloques de agua
	for block := range waterChan {
		_, err := c.Writer.Write([]byte(block))
		if err != nil {
			fmt.Println("Error escribiendo al cliente:", err)
			break
		}
		// Asegurar que se envíen los datos inmediatamente
		c.Writer.Flush()
	}
}

func main() {
	r := gin.Default()

	r.GET("/water", func(c *gin.Context) {
		// Leer el parámetro "quantity" de la URL
		quantityStr := c.Query("quantity")
		if quantityStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'quantity' es requerido"})
			return
		}

		// Convertir el parámetro a entero
		quantity, err := strconv.Atoi(quantityStr)
		if err != nil || quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El parámetro 'quantity' debe ser un número entero positivo"})
			return
		}

		// Cada cantidad genera 10 unidades de agua, calcular cuántos bloques enviar
		blocks := quantity

		// Llamar a la función para entregar el agua en bloques
		deliverWater(c, blocks)
	})

	// Ejecutar el servidor en el puerto 4005
	r.Run(":4005")
}
