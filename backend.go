package main

import (
	"context"
	"fmt"
	"ica-caldav/ica"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

func NewIcaBackend(ica *ica.ICA) *ICABackend {
	return &ICABackend{
		ica: ica,
	}
}

type ICABackend struct {
	ica *ica.ICA
}

func (be *ICABackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return "/user/", nil
}

func (be *ICABackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return "/user/shoppinglists/", nil
}

func (be *ICABackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	lists, err := be.ica.GetShoppingLists()
	if err != nil {
		return nil, err
	}
	var calendars = make([]caldav.Calendar, 0)
	for _, list := range lists {
		calendars = append(calendars, createCalendar(list))
	}
	return calendars, nil
}

func (be *ICABackend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	list, err := be.getList(ctx, path)
	if err != nil {
		return nil, err
	}
	cal := createCalendar(*list)
	return &cal, nil
}

func (be *ICABackend) ListCalendarObjects(ctx context.Context, path string, req *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	list, err := be.getList(ctx, path)
	if err != nil {
		return nil, err
	}
	calendarObjects := make([]caldav.CalendarObject, 0)
	for _, row := range list.Rows {
		calendarObjects = append(calendarObjects,
			createCalendarObject(
				row,
				fmt.Sprintf("%s%s", path, row.Id),
			),
		)
	}
	slog.Info("Listing objects",
		"list", list.Name,
		"count", len(calendarObjects),
	)
	return calendarObjects, nil
}

func (be *ICABackend) GetCalendarObject(ctx context.Context, path string, req *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	listPath, id := filepath.Split(path)
	list, err := be.getList(ctx, listPath)
	if err != nil {
		slog.Error("Could not find list",
			"path", listPath,
		)
		return nil, err
	}
	for _, row := range list.Rows {
		if row.Id == id {
			cal := createCalendarObject(row, path)
			return &cal, nil
		}
	}
	slog.Error("Could not find item",
		"path", path,
	)
	return nil, fmt.Errorf("Not found")
}

func (be *ICABackend) PutCalendarObject(ctx context.Context, path string, calendar *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (obj *caldav.CalendarObject, err error) {
	todo, err := getTodo(calendar)
	if err != nil {
		return nil, err
	}

	name, err := todo.Props.Text(ical.PropSummary)
	if err != nil {
		return nil, err
	}
	id, err := todo.Props.Text(ical.PropUID)
	if err != nil {
		return nil, err
	}

	listPath, _ := filepath.Split(path)
	list, err := be.getList(ctx, listPath)
	if err != nil {
		return nil, err
	}
	for _, row := range list.Rows {
		if row.Id == id {
			// This just means something (Apple reminder) tries to update the items (for some reason).
			// We just ignore these cases
			cal := createCalendarObject(row, path)
			return &cal, nil
		}
	}

	completed, _ := todo.Props.DateTime(ical.PropCompleted, time.Local)
	if !completed.IsZero() {
		// We don't want to add already completed items
		return nil, fmt.Errorf("Adding completed items isn't supported")
	}

	toAdd := ica.ItemToAdd{
		Name: name,
	}
	row, err := be.ica.AddItem(*list, toAdd)
	if err != nil {
		return nil, err
	}
	newPath := fmt.Sprintf("%v/%v", listPath, row.Id)
	cal := createCalendarObject(*row, newPath)
	return &cal, nil
}

// Cache
type ListCache struct {
	ica   *ica.ICA
	Lists []ica.ShoppingList
}

func (c *ListCache) GetShoppingLists() ([]ica.ShoppingList, error) {
	if len(c.Lists) > 0 {
		return c.Lists, nil
	}
	lists, err := c.ica.GetShoppingLists()
	c.Lists = lists
	return lists, err
}

// Utilities
func (be *ICABackend) getList(ctx context.Context, path string) (*ica.ShoppingList, error) {

	// Try using pre-fetched lists from context first
	listCache, ok := ctx.Value("listCache").(ListCache)
	if !ok {
		panic("")
	}
	lists, err := listCache.GetShoppingLists()
	if err != nil {
		return nil, err
	}

	id, err := filepath.Rel("/user/shoppinglists/", path)
	if err != nil {
		return nil, err
	}
	for _, list := range lists {
		if list.Id == id {
			return &list, nil
		}
	}
	return nil, fmt.Errorf("Not Found")
}

func createCalendar(list ica.ShoppingList) caldav.Calendar {
	return caldav.Calendar{
		Path:                  fmt.Sprintf("/user/shoppinglists/%s/", list.Id),
		Name:                  list.Name,
		MaxResourceSize:       1000,
		SupportedComponentSet: []string{"VTODO"},
	}
}

func createCalendarObject(row ica.ShoppingListRow, path string) caldav.CalendarObject {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//xyz Corp//NONSGML PDA Calendar Version 1.0//EN")
	cal.Children = []*ical.Component{
		createEvent(row).Component,
	}
	return caldav.CalendarObject{
		Path:    path,
		Data:    cal,
		ModTime: row.Updated,
		ETag:    row.ETag(),
	}
}

func createEvent(row ica.ShoppingListRow) ical.Event {
	event := ical.NewEvent()
	event.Name = ical.CompToDo
	event.Props.SetText(ical.PropUID, row.Id)
	event.Props.SetDateTime(ical.PropDateTimeStamp, row.Updated)
	event.Props.SetText(ical.PropSummary, row.Name)
	event.Props.SetText(ical.PropDescription, "")
	if row.IsStriked {
		// We just assume that it was striked out when last updated
		event.Props.SetDateTime(ical.PropCompleted, row.Updated)
	}
	return *event
}

func getTodo(calendar *ical.Calendar) (*ical.Component, error) {
	for _, child := range calendar.Children {
		if child.Name == ical.CompToDo {
			return child, nil
		}
	}
	return nil, fmt.Errorf("Unsupported number of children")
}

// Not implemented, but required by interface
func (be *ICABackend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return fmt.Errorf("Not implemented")
}

func (be *ICABackend) DeleteCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return fmt.Errorf("Not implemented")
}

func (be *ICABackend) DeleteCalendarObject(ctx context.Context, path string) error {
	return fmt.Errorf("Not implemented")
}

func (be *ICABackend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	return nil, fmt.Errorf("Not implemented")
}
