package main

import (
	"strings"
)

var currentProject *Project

type Project struct {
	Open bool
	path string
}

func NewProject(path string) *Project {
	currentProject = new(Project)
	currentProject.Open = true
	currentProject.path = path
	if !strings.HasSuffix(path, ".orq") {
		path += ".orq"
	}
	NewDataContext(path).Close()
	return currentProject
}

func (proj *Project) GetData() *DataContext {
	return NewDataContext(proj.path)
}
