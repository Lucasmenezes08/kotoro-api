package subject

import (
	"time"
	"github.com/google/uuid"
)


type Color string

const (
	ColorGreen  Color = "green"
	ColorRed    Color = "red"
	ColorBlue   Color = "blue"
	ColorOrange Color = "orange"
	ColorBlack  Color = "black"
	ColorGray   Color = "gray"
	ColorPurple Color = "purple"
	ColorWhite  Color = "white"
	ColorPink   Color = "pink"
	ColorYellow Color = "yellow"
)


type Subject struct {
	Id uuid.UUID `db:"id"`
	Name string `db:"name"`
	Color Color `db:"color"` 
	Created_at time.Time `db:"created_at"`
	Updated_at time.Time `db:"updated_at"`
	Deleted_at *time.Time `db:"deleted_at"`
} 