package main

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

type ClotheOrderSize int8

const (
	small ClotheOrderSize = 3
	medium ClotheOrderSize = 6
	big ClotheOrderSize = 9
)

const waiting = "waiting"

type Order struct{
	clothesOrderSize ClotheOrderSize
	initTime time.Time
	endTime *ime.Time 
}

type CreateOrderRequest struct {
	
}

func newOrder(clothesOrderSize int8) (*Order, error){
	if clothesOrderSize != 1 || clothesOrderSize != 2 || clothesOrderSize != 3 {
		return nil, errors.New("invalid clothes order size")
	}
	var size ClotheOrderSize
	switch clothesOrderSize {
	case 1:
		size = small
	case 2:
		size = medium
	case 3:
		size = big
	}
	return &Order{clothesOrderSize: size, initTime: time.Now(), endTime: nil}, nil
}

func main() {
	r := gin.Default()

	r.POST()

}