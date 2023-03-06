package model

type MessageType int

const (
	MTEmpty      MessageType = 0
	MTState      MessageType = 1
	MTStateDelta MessageType = 2
	MTSpeedmap   MessageType = 3
	MTCar        MessageType = 4
)
