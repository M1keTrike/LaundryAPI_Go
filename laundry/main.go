package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type LaundryOrder struct {
	ID             int
	LoadType       int
	StartTime      time.Time
	EndTime        time.Time
	Priority       int
	AssignedWasher string
	Status         string
}

type LaundryServer struct {
	orders     []*LaundryOrder
	orderMutex sync.Mutex
	orderID    int
	washerURL  string
	waitQueue  chan *LaundryOrder
}

const (
	WasherServerURL = "http://localhost:4007/start"
	MaxQueueSize    = 100
)

func NewLaundryServer() *LaundryServer {
	return &LaundryServer{
		orders:    []*LaundryOrder{},
		washerURL: WasherServerURL,
		waitQueue: make(chan *LaundryOrder, MaxQueueSize),
	}
}

func (ls *LaundryServer) AddOrder(loadType int, priority int) *LaundryOrder {
	ls.orderMutex.Lock()
	defer ls.orderMutex.Unlock()

	ls.orderID++
	order := &LaundryOrder{
		ID:       ls.orderID,
		LoadType: loadType,
		Priority: priority,
		Status:   "Pendiente",
	}
	ls.orders = append(ls.orders, order)

	// Agregar a la cola de espera para ser procesada
	ls.waitQueue <- order
	return order
}

func (ls *LaundryServer) processOrders() {
	for order := range ls.waitQueue {
		ls.assignOrderToWasher(order)
	}
}

func (ls *LaundryServer) assignOrderToWasher(order *LaundryOrder) {
	query := fmt.Sprintf("%s?load=%d", ls.washerURL, order.LoadType)
	resp, err := http.Get(query)
	if err != nil {
		fmt.Printf("Error al enviar la orden ID %d al servidor de lavadoras: %v\n", order.ID, err)
		order.Status = "Error"
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		order.StartTime = time.Now()
		order.Status = "En Proceso"

		done := make(chan string)
		go func() {
			defer close(done)

			var response map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				done <- fmt.Sprintf("Error al completar la orden ID %d: %v", order.ID, err)
				return
			}

			message, ok := response["message"].(string)
			if !ok {
				done <- fmt.Sprintf("Respuesta inesperada del servidor de lavadoras para la orden ID %d", order.ID)
				return
			}

			done <- message
		}()

		// Esperar la confirmación de finalización
		message := <-done
		order.EndTime = time.Now()
		order.Status = "Completado"
		order.AssignedWasher = "Lavadora asignada"
		fmt.Printf("Orden ID %d finalizada con éxito. Mensaje: %s\n", order.ID, message)
	} else {
		fmt.Printf("No se pudo asignar la orden ID %d, reintentando más tarde.\n", order.ID)
		order.Status = "Pendiente"
		ls.waitQueue <- order // Reagregar a la cola si falla
	}
}

func (ls *LaundryServer) GetOrders() []*LaundryOrder {
	ls.orderMutex.Lock()
	defer ls.orderMutex.Unlock()
	return ls.orders
}

func (ls *LaundryServer) GetOrderByID(id int) (*LaundryOrder, bool) {
	ls.orderMutex.Lock()
	defer ls.orderMutex.Unlock()

	for _, order := range ls.orders {
		if order.ID == id {
			return order, true
		}
	}
	return nil, false
}

func main() {
	laundryServer := NewLaundryServer()

	// Iniciar procesamiento de órdenes en la cola
	go laundryServer.processOrders()

	r := gin.Default()

	// Endpoint para crear una nueva orden
	r.POST("/order", func(c *gin.Context) {
		loadTypeStr := c.Query("loadType")
		priorityStr := c.Query("priority")
		if loadTypeStr == "" || priorityStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Los parámetros 'loadType' y 'priority' son requeridos"})
			return
		}

		loadType, err1 := strconv.Atoi(loadTypeStr)
		priority, err2 := strconv.Atoi(priorityStr)
		if err1 != nil || err2 != nil || loadType < 1 || loadType > 3 || priority < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetros inválidos"})
			return
		}

		order := laundryServer.AddOrder(loadType, priority)

		// Esperar a que la orden sea procesada
		message := make(chan string)
		go func() {
			<-message // Bloquear hasta que la orden termine
		}()

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Orden ID %d en cola", order.ID),
			"details": gin.H{
				"order_id": order.ID,
				"status":   order.Status,
			},
		})
	})

	// Endpoint para listar todas las órdenes
	r.GET("/orders", func(c *gin.Context) {
		orders := laundryServer.GetOrders()
		c.JSON(http.StatusOK, orders)
	})

	// Endpoint para obtener una orden específica
	r.GET("/order/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID inválido"})
			return
		}

		order, found := laundryServer.GetOrderByID(id)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Orden no encontrada"})
			return
		}

		c.JSON(http.StatusOK, order)
	})

	r.Run(":4010")
}
