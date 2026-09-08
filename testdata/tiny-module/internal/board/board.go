// Package board holds JSON data types for the board. No domain logic.
package board

type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
