package subject

import (
	"github.com/google/uuid"
	"time"
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
	Id         uuid.UUID  `db:"id" json:"id"`
	UserId     uuid.UUID  `db:"user_id" json:"user_id"`
	Name       string     `db:"name" json:"name"`
	Color      Color      `db:"color" json:"color"`
	Created_at time.Time  `db:"created_at" json:"created_at"`
	Updated_at time.Time  `db:"updated_at" json:"updated_at"`
	Deleted_at *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}
