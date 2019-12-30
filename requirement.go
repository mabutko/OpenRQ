package main

import (
	"crypto/md5"
	"fmt"
	"os"
)

// Requirement with ItemProprties
type Requirement struct {
	Item
	id int64
}

func NewRequirement(id int64) Requirement {
	req := Requirement{}
	req.id = id
	if req.IsNull() {
		fmt.Fprintln(os.Stderr, "warning: requirement", id, "not found")
	}
	return req
}

func (req Requirement) IsNull() bool {
	var count int
	req.GetValue("count(*)", &count)
	return count <= 0
}

// GetChildren get all children of requirement
func (req Requirement) Children() []Item {
	return nil
}

// RemoveChild remove specified child of requirement
func (req Requirement) RemoveChild(child Item) {
}

// GetHash get hash of requirement
func (req Requirement) Hash() [16]byte {
	return md5.Sum([]byte(fmt.Sprintf("%v", req)))
}

// GetValue gets a value from the database
func (req *Requirement) GetValue(name string, value interface{}) {
	db := currentProject.Data()
	defer db.Close()
	err := db.GetItemValue(req.ID(), "Requirements", name, value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database error:", err)
	}
}

func (req *Requirement) GetValueString(name string) string {
	var val string
	req.GetValue(name, &val)
	return val
}

func (req *Requirement) GetValueInt(name string) int {
	var val int
	req.GetValue(name, &val)
	return val
}

func (req *Requirement) GetValueInt64(name string) int64 {
	var val int64
	req.GetValue(name, &val)
	return val
}

// SetValue sets a value to the database
func (req *Requirement) SetValue(name string, value interface{}) {
	db := currentProject.Data()
	defer db.Close()
	db.SetItemValue(req.ID(), "Requirements", name, value)
}

// GetRationale gets the rationale property of the requirement
func (req *Requirement) Rationale() string {
	return req.GetValueString("rationale")
}

// GetFitCriterion of Requirement
func (req *Requirement) FitCriterion() string {
	return req.GetValueString("fitCriterion")
}

// GetId gets the row ID in the database
func (req Requirement) ID() int64 {
	return req.id
}

// GetUid gets the row Uid in the database
func (req Requirement) UID() int64 {
	return req.GetValueInt64("uid")
}

// SetUid sets the Uid in the database
func (req Requirement) SetUID(uid int64) {
	req.SetValue("uid", uid)
}

// GetVersion of Requirement
func (req Requirement) Version() int {
	return req.GetValueInt("version")
}

// GetShown gets the root as hidden or shown
func (req Requirement) Shown() bool {
	var val bool
	req.GetValue("shown", &val)
	return val
}

// SetShown sets the root as hidden or shown
func (req Requirement) SetShown(shown bool) {
	req.SetValue("shown", shown)
}

// GetDescription gets the description from the database
func (req Requirement) Description() string {
	return req.GetValueString("description")
}

// AddChild
func (req Requirement) AddChild(child Item) {

}

func (req Requirement) Pos() (int, int) {
	var x, y int
	req.GetValue("x", &x)
	req.GetValue("y", &y)
	return x, y
}

func (req Requirement) SetPos(x, y int) {
	req.SetValue("x", x)
	req.SetValue("y", y)
}

func (req Requirement) Size() (int, int) {
	var width, height int
	req.GetValue("width", &width)
	req.GetValue("height", &height)
	return width, height
}

func (req Requirement) SetSize(w, h int) {
	req.SetValue("width", w)
	req.SetValue("height", h)
}