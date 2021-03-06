package main

import (
	"fmt"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
	"os"
	"path/filepath"
	"strings"
)

type Link struct {
	parent Item
	child  Item
	line   *widgets.QGraphicsLineItem
	dir    *widgets.QGraphicsPolygonItem
}

func (link Link) SetChildItem(child Item) {
	childItemID := core.NewQVariant1(child.ID())
	childItemType := core.NewQVariant1(int(GetItemType(child)))
	link.line.SetData(0, childItemID)
	link.line.SetData(1, childItemType)
	link.dir.SetData(0, childItemID)
	link.dir.SetData(1, childItemType)
}

var links map[Item][]*Link

var view *widgets.QGraphicsView
var scene *widgets.QGraphicsScene

var backgroundColor *gui.QColor

// Items opened in an edit window
var openItems map[Item]*widgets.QDockWidget

func IsItemOpen(item Item) bool {
	_, ok := openItems[item]
	return ok
}

func CloseItem(item Item) {
	delete(openItems, item)
}

func ReloadProject(window *widgets.QMainWindow) {
	// Make sure view is enabled
	view.SetEnabled(true)
	// Delete all current items
	scene.Clear()
	// Clear links
	links = make(map[Item][]*Link)
	// Close all open items
	for id := range openItems {
		openItems[id].Close()
		CloseItem(id)
	}
	// Connect to database to get new items
	db := currentProject.Data()
	defer db.Close()
	// Load items
	items, err := db.Items()
	if err != nil {
		fmt.Println("error: failed to get saved items:", err)
	} else {
		var x, y, w, h int
		for item, description := range items {
			x, y = item.Pos()
			w, h = item.Size()
			scene.AddItem(NewGraphicsItem(description, x, y, w, h, item))
		}
	}
	// Load links
	links, err := db.Links()
	if err != nil {
		fmt.Println("error: failed to get saved links:", err)
	} else {
		for child, parent := range links {
			// Find parent and child
			var parentItem, childItem *widgets.QGraphicsItemGroup
			for _, item := range view.Items() {
				group := item.Group()
				if group == nil {
					continue
				}
				groupID := group.Data(0).ToLongLong(nil)
				groupType := ItemType(group.Data(1).ToInt(nil))
				if groupID == 0 || groupType == 0 {
					continue
				}
				if groupID == child.ID() && groupType == GetItemType(child) {
					childItem = group
				} else if groupID == parent.ID() && groupType == GetItemType(parent) {
					parentItem = group
				}
				// Stop loop if we found everything
				if parentItem != nil && childItem != nil {
					break
				}
			}
			// Create and add link
			if parentItem == nil || childItem == nil {
				fmt.Printf("warning: could not find parent or child, ignoring link (%3v(%v)%v -> %3v(%v)%v)\n",
					parent.ID(), GetItemType(parent), parentItem != nil, child.ID(), GetItemType(child), childItem != nil)
			} else {
				link := CreateLink(parentItem, childItem)
				scene.AddItem(link.line)
				scene.AddItem(link.dir)
			}
		}
	}
	// Set window title
	UpdateWindowTitle(window)
}

func UpdateWindowTitle(window *widgets.QMainWindow) {
	abs, err := filepath.Abs(currentProject.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: failed to get absolute path to project:", err)
		abs = currentProject.path
	}
	window.SetWindowTitle(fmt.Sprintf("%v [%v] - OpenRQ", currentProject.Data().ProjectName(), abs))
	// Update last used project
	// (should probably not be done here)
	NewSettings().SetLastProject(abs)
}

// SnapToGrid naps the specified position to the grid
func SnapToGrid(pos *core.QPoint) *core.QPoint {
	// 2^5=32
	const gridSize = 5
	scenePos := view.MapToScene(pos).ToPoint()
	return view.MapFromScene(core.NewQPointF3(
		float64((scenePos.X()>>gridSize<<gridSize)-64), float64((scenePos.Y()>>gridSize<<gridSize)-32)))
}

func CreateEditWidgetFromPos(pos core.QPoint_ITF, scene *widgets.QGraphicsScene) (*widgets.QDockWidget, bool) {
	// Get UID
	group := view.ItemAt(pos).Group()
	item := GetGroupItem(group)
	// Check if already opened
	if IsItemOpen(item) {
		// We probably want to put it in focus here or something
		return nil, false
	}
	// Open item
	editWindow := CreateEditWidget(item, group, scene)
	editWindow.ConnectCloseEvent(func(event *gui.QCloseEvent) {
		CloseItem(item)
	})
	// Set item as being opened
	openItems[item] = editWindow
	// Return new window
	return editWindow, true
}

func CreateView(window *widgets.QMainWindow, linkBtn *widgets.QToolButton) *widgets.QGraphicsView {
	// Create scene and view
	scene = widgets.NewQGraphicsScene(nil)
	view = widgets.NewQGraphicsView2(scene, nil)
	// Get default window background color
	backgroundColor = window.Palette().Color2(window.BackgroundRole())
	// Create open items map
	openItems = make(map[Item]*widgets.QDockWidget)

	// Check if we have a last loaded project
	if project := NewSettings().LastProject(); project != "" {
		NewProject(project)
		ReloadProject(window)
	} else {
		// No recent project, show message
		font := gui.NewQFont()
		font.SetPointSize(18)
		text := scene.AddText("No Project Loaded", font)
		text.SetX(float64(view.Width() / 2) + (scene.Width() / 2.0))
		text.SetY(float64(view.Height() / 2) + scene.Height())
		scene.SetSceneRect2(0, 0, float64(view.Width()), float64(view.Height()))
		view.SetEnabled(false)
	}

	// Setup drag-and-drop
	view.SetAcceptDrops(true)
	view.SetAlignment(core.Qt__AlignTop | core.Qt__AlignLeft)
	view.ConnectDragMoveEvent(func(event *gui.QDragMoveEvent) {
		if event.Source() != nil && event.Source().IsWidgetType() {
			event.AcceptProposedAction()
		}
	})
	// What item we're currently moving, if any
	var movingItem *widgets.QGraphicsItemGroup
	// Start position of link
	var linkStart *widgets.QGraphicsItemGroup
	// Temporary line shown when creating a new link
	var tempLink *widgets.QGraphicsLineItem

	itemSize := 64
	view.ConnectDropEvent(func(event *gui.QDropEvent) {
		pos := view.MapToScene(event.Pos())

		// Add item to database
		// For now, we assume all items are requirements
		db := currentProject.Data()
		defer db.Close()
		uid, err := db.AddEmptyRequirement()
		if err != nil {
			widgets.QMessageBox_Warning(
				window, "Failed to add item", err.Error(),
				widgets.QMessageBox__Ok, widgets.QMessageBox__NoButton)
			return
		}
		// Snap to grid
		gridPos := SnapToGrid(pos.ToPoint())
		// Set size and position
		// All items are requirements by default
		req := NewRequirement(uid)
		req.SetPos(gridPos.Y(), gridPos.Y())
		req.SetSize(itemSize*2, itemSize)
		// Add item to view
		scene.AddItem(NewGraphicsItem(req.Description(), gridPos.X(), gridPos.Y(), itemSize*2, itemSize, req))
		if len(openItems) <= 0 {
			openItems[req], _ = CreateEditWidgetFromPos(event.Pos(), scene)
			window.AddDockWidget(core.Qt__RightDockWidgetArea, openItems[req])
		}
	})

	view.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		if event.Button() != core.Qt__LeftButton {
			return
		}
		item := view.ItemAt(event.Pos())
		// If an item was found
		if item != nil && item.Group() != nil {
			if linkBtn.IsChecked() {
				// We're creating a link
				linkStart = item.Group()
				// Create temporary link indicator
				scenePos := view.MapToScene(event.Pos())
				tempLink = widgets.NewQGraphicsLineItem2(core.NewQLineF2(scenePos, scenePos), nil)
				tempLink.SetPen(gui.NewQPen3(gui.NewQColor3(0, 255, 0, 128)))
				scene.AddItem(tempLink)
			} else {
				// We're moving an item
				movingItem = item.Group()
				movingItem.SetOpacity(0.6)
			}
		}
	})
	view.ConnectMouseMoveEvent(func(event *gui.QMouseEvent) {
		if movingItem != nil {
			// Update item position
			movingItem.SetPos(view.MapToScene(SnapToGrid(event.Pos())))
			// Update link
			UpdateLinkPos(movingItem, movingItem.Pos().X(), movingItem.Pos().Y())
		}
		// Update temporary link
		if tempLink != nil {
			tempLine := tempLink.Line()
			tempLine.SetP2(view.MapToScene(event.Pos()))
			tempLink.SetLine(tempLine)
		}
		// Show hand when trying to move item
		if !linkBtn.IsChecked() && view.ItemAt(event.Pos()).Group() != nil && view.ItemAt(event.Pos()).Group().Type() != 0 {
			cursor := core.Qt__OpenHandCursor
			// If moving, show closed hand
			if movingItem != nil {
				cursor = core.Qt__ClosedHandCursor
			}
			view.SetCursor(gui.NewQCursor2(cursor))
		} else {
			// If not hovering over an item, restore cursor
			view.UnsetCursor()
		}
	})
	view.ConnectMouseReleaseEvent(func(event *gui.QMouseEvent) {
		if event.Button() == core.Qt__RightButton && view.ItemAt(event.Pos()).Group() != nil {
			pos := event.Pos()
			menu := widgets.NewQMenu(nil)
			// Check if clicking on link
			item := view.ItemAt(pos)
			group := item.Group()
			// If type is 0, it's probably a link
			if group.Type() == 0 {
				// We hopefully clicked a link
				menu.AddAction2(GetIcon("menu-delete"), "Delete").ConnectTriggered(func(checked bool) {
					childItem := NewItem(item.Data(0).ToLongLong(nil), ItemType(item.Data(1).ToInt(nil)))
					// Remove in front end
					for _, itemLinks := range links {
						for _, link := range itemLinks {
							if link.child == childItem {
								// Remove from database
								childItem.SetParent(nil)
								// Remove from graphics scene
								scene.RemoveItem(link.line)
								scene.RemoveItem(link.dir)
								// Remove from links map
								RemoveLink(link)
								// Assume deleting one link
								return
							}
						}
					}
				})
				menu.Popup(view.MapToGlobal(pos), nil)
				return
			}
			// Edit option
			menu.AddAction2(GetIcon("menu-edit"), "Edit").
				ConnectTriggered(func(checked bool) {
					if editWidget, ok := CreateEditWidgetFromPos(pos, scene); ok {
						window.AddDockWidget(core.Qt__RightDockWidgetArea, editWidget)
					}
				})
			// Delete option
			menu.AddAction2(GetIcon("menu-delete"), "Delete").
				ConnectTriggered(func(checked bool) {
					// Connect to database
					db := currentProject.Data()
					defer db.Close()
					// Get the clicked item
					group := view.ItemAt(pos).Group()
					item := GetGroupItem(group)
					// Try to get all links
					link, ok := links[item]
					if ok {
						// Remove all links
						for _, l := range link {
							// It is the item we are trying to remove
							if l.parent.ID() == item.ID() || l.child.ID() == item.ID() {
								// Remove from scene
								scene.RemoveItem(l.line)
								scene.RemoveItem(l.dir)
								// Remove from links map
								RemoveLink(l)
							}
						}
					}
					// Remove the group from the scene
					scene.RemoveItem(group)
					// Remove the links and the item itself from the database
					if err := db.RemoveChildrenLinks(item); err != nil {
						fmt.Println("warning: failed to remove children links:", err)
					}
					if err := db.RemoveItem(item); err != nil {
						fmt.Println("warning: failed to remove item:", err)
					}
					// Check if item is opened in editor
					if openItem, ok := openItems[item]; ok {
						openItem.Close()
						delete(openItems, item)
					}
				})
			// Show menu at cursor
			menu.Popup(view.MapToGlobal(event.Pos()), nil)
			return
		}

		// We released a button while moving an item
		if movingItem != nil {
			pos := movingItem.Pos()
			// Update link if needed
			// Error handling is already taken care of in UpdateLinkPos
			UpdateLinkPos(movingItem, pos.X(), pos.Y())
			// Update position in database
			GetGroupItem(movingItem).SetPos(int(pos.X()), int(pos.Y()))
			// Reset opacity and remove as moving
			movingItem.SetOpacity(1.0)
			movingItem = nil
		}
		// When releasing, we always want to destroy temp link
		if tempLink != nil {
			scene.RemoveItem(tempLink)
			tempLink = nil
		}
		// We released while creating a link
		if linkStart != nil {
			linkStartItem := GetGroupItem(linkStart)
			group := view.ItemAt(event.Pos()).Group()
			groupItem := GetGroupItem(group)
			// If we try to link to the empty void or self
			if group == nil || linkStartItem == nil || (linkStartItem.ID() == groupItem.ID() &&
				GetItemType(linkStartItem) == GetItemType(groupItem)) {
				linkStart = nil
				return
			}
			toPos := group.Pos()
			if toPos.X() == 0 && toPos.Y() == 0 {
				return
			}
			// Create and add link
			link := CreateLink(linkStart, group)
			scene.AddItem(link.line)
			scene.AddItem(link.dir)
			linkStart = nil
			// Add link to database
			db := currentProject.Data()
			// Check if child already have a parent and add to db if it doesn't have one
			if !GetGroupItem(group).IsPropertyNull("parent") {
				fmt.Println("warning: child already has a parent")
			} else if err := db.AddItemChild(linkStartItem, groupItem); err != nil {
				fmt.Println("error: failed to add link to database:", err)
			}
			// We're done
			db.Close()
		}
	})
	return view
}

func GetGroupItem(group *widgets.QGraphicsItemGroup) Item {
	// Check if group is nil
	if group == nil {
		fmt.Println("error: no group to create item from")
		return nil
	}
	// Get ID and type
	itemID := group.Data(0).ToLongLong(nil)
	itemType := ItemType(group.Data(1).ToInt(nil))
	// If either is zero, something is wrong
	if itemID == 0 || itemType == 0 {
		fmt.Println("error: could not create item from id", itemID, "type", itemType)
		return nil
	}
	// Create a new type and return it
	return NewItem(itemID, itemType)
}

func CreateLink(parent, child *widgets.QGraphicsItemGroup) Link {
	// Check if map needs to be created
	if links == nil {
		links = make(map[Item][]*Link)
	}
	// Check if we're linking to self
	parentItem := GetGroupItem(parent)
	childItem := GetGroupItem(child)
	// Get from (parent) and to (child)
	fromPos := parent.Pos()
	toPos := child.Pos()
	// Create graphics line
	line := widgets.NewQGraphicsLineItem3(
		fromPos.X()+64, fromPos.Y()+32,
		toPos.X()+64, toPos.Y()+32,
		nil,
	)
	// Set the color of it
	line.SetPen(gui.NewQPen3(gui.NewQColor3(0, 255, 0, 255)))
	// Create line data
	lineData := Link{
		parentItem, childItem, line,
		CreateTriangle(line.Line().Center(), line.Line().Angle()),
	}
	// Set data in line and triangle
	lineData.SetChildItem(childItem)
	// Save in links map
	links[parentItem] = append(links[parentItem], &lineData)
	links[childItem] = append(links[childItem], &lineData)
	// Return the graphics line to add to scene
	return lineData
}

func UpdateLinkPos(item *widgets.QGraphicsItemGroup, x, y float64) {
	// Get link
	itemID := GetGroupItem(item)
	link, ok := links[itemID]
	// Error checking
	if !ok {
		return
	}
	for _, l := range link {
		// If the item is the parent
		isParent := l.parent == itemID
		// Update position of either parent or child
		if isParent {
			pos := l.line.Line().P2()
			l.line.SetLine2(x+64, y+32, pos.X(), pos.Y())
		} else {
			pos := l.line.Line().P1()
			l.line.SetLine2(pos.X(), pos.Y(), x+64, y+32)
		}
		// Update direction
		center := l.line.Line().Center()
		l.dir.SetPos2(center.X()-8, center.Y()-8)
		l.dir.SetRotation((-l.line.Line().Angle()) - 90)
	}
}

func RemoveLink(link *Link) {
	// Remove from child
	delete(links, link.child)
	// Remove from parent
	for i, childLink := range links[link.parent] {
		if childLink.child == link.child {
			last := len(links[link.parent])-1
			// Replace entry to delete with last
			links[link.parent][i] = links[link.parent][last]
			// Cut away last element
			links[link.parent] = links[link.parent][:last]
			break
		}
	}
}

func NewGraphicsItem(text string, x, y, width, height int, item Item) *widgets.QGraphicsItemGroup {
	group := widgets.NewQGraphicsItemGroup(nil)
	textItem := widgets.NewQGraphicsTextItem(nil)
	// Check if plain text is too long
	doc := gui.NewQTextDocument(nil)
	doc.SetHtml(text)
	const maxLength = 46
	if len(doc.ToPlainText()) > maxLength {
		cursor := gui.NewQTextCursor2(doc)
		// Move to end
		cursor.MovePosition(gui.QTextCursor__End, gui.QTextCursor__MoveAnchor, 1)
		// Keep removing when too long
		for len(doc.ToPlainText()) > maxLength {
			cursor.DeletePreviousChar()
		}
		// Remove last word to avoid weird cropping
		cursor.Select(gui.QTextCursor__WordUnderCursor)
		if len(cursor.Selection().ToPlainText()) != len(doc.ToPlainText()) {
			cursor.InsertText("")
		}
		cursor.ClearSelection()
		// Remove all trailing spaces
		cursor.MovePosition(gui.QTextCursor__End, gui.QTextCursor__MoveAnchor, 1)
		for strings.HasSuffix(doc.ToPlainText(), " ") && len(doc.ToPlainText()) > 0 {
			cursor.DeletePreviousChar()
		}
		// Insert ... at end
		cursor.InsertText("...")
	}
	// If no description, set text to item string
	if len(doc.ToPlainText()) <= 0 {
		doc.SetHtml(fmt.Sprintf("<small>(%v)</small>", item.ToString()))
	}
	textItem.SetHtml(doc.ToHtml(core.NewQByteArray()))
	textItem.SetZValue(15)
	textItem.SetTextWidth(float64(width))
	shapeItem := widgets.NewQGraphicsRectItem3(0, 0, float64(width), float64(height), nil)
	shapeItem.SetBrush(gui.NewQBrush3(backgroundColor, 1))
	// Default purple color for requirements
	var penColor uint = 10233776
	// Blue color for solutions
	if GetItemType(item) == TypeSolution {
		penColor = 2201331
	}
	// Set opacity of border color
	color := gui.NewQColor4(penColor)
	color.SetAlpha(200)
	shapeItem.SetPen(gui.NewQPen3(color))
	group.AddToGroup(textItem)
	group.AddToGroup(shapeItem)
	group.SetPos2(float64(x), float64(y))
	group.SetData(0, core.NewQVariant1(item.ID()))
	group.SetData(1, core.NewQVariant1(int(GetItemType(item))))
	group.SetZValue(10)
	return group
}

// CreateTriangle creates a new 16x16 triangle pointing downwards
func CreateTriangle(pos *core.QPointF, angle float64) *widgets.QGraphicsPolygonItem {
	// Total width/height for triangle
	const size = 16
	// Create each point
	points := []*core.QPointF{
		core.NewQPointF3(0, 0),
		core.NewQPointF3(size, 0),
		core.NewQPointF3(size>>1, size),
	}
	// Create polygon and return it
	poly := widgets.NewQGraphicsPolygonItem2(gui.NewQPolygonF3(points), nil)
	poly.SetPos2(pos.X()-(size>>1), pos.Y()-(size>>1))
	poly.SetPen(gui.NewQPen3(gui.NewQColor3(0, 255, 0, 255)))
	poly.SetTransformOriginPoint2(size>>1, size>>1)
	poly.SetRotation((-angle) - 90)
	return poly
}

func Roots() []Item {
	// Final tree
	roots := make([]Item, 0)
	// Items we have already added
	added := map[Item]int{}
	// Loop through all items in view
	for _, item := range view.Items() {
		// Get group and make sure it's valid
		group := item.Group()
		if group == nil || group.Type() == 0 {
			continue
		}
		// Get item
		groupItem := GetGroupItem(group)
		// Ignore if already added
		if ContainsItem(added, groupItem) {
			continue
		}
		added[groupItem] = 0

		isRoot := true
		for _, l2 := range links[groupItem] {
			if l2.child == groupItem {
				isRoot = false
				break
			}
		}
		if isRoot {
			roots = append(roots, groupItem)
		}
	}
	return roots
}