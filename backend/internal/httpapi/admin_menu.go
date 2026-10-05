package httpapi

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/gravizz/freshket/backend/internal/menu"
	"github.com/gravizz/freshket/backend/internal/pricing"
)

type adminItemDTO struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`
	Active bool   `json:"active"`
}

// itemError maps repository errors to HTTP statuses.
func itemError(err error) error {
	switch {
	case errors.Is(err, menu.ErrDuplicate):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, menu.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}
	return err
}

func (h *handler) listAllItems(c fiber.Ctx) error {
	items, err := h.menu.ListAll(c.Context())
	if err != nil {
		return err
	}
	out := make([]adminItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, adminItemDTO{Code: it.Code, Name: it.Name, Price: int64(it.Price), Active: it.Active})
	}
	return c.JSON(out)
}

func (h *handler) createItem(c fiber.Ctx) error {
	var body adminItemDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	it := pricing.Item{Code: body.Code, Name: body.Name, Price: pricing.Money(body.Price), Active: body.Active}
	if err := it.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.menu.Create(c.Context(), it); err != nil {
		return itemError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(body)
}

func (h *handler) updateItem(c fiber.Ctx) error {
	var body adminItemDTO
	if err := c.Bind().JSON(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	body.Code = c.Params("code")
	it := pricing.Item{Code: body.Code, Name: body.Name, Price: pricing.Money(body.Price), Active: body.Active}
	if err := it.Validate(); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if err := h.menu.Update(c.Context(), it); err != nil {
		return itemError(err)
	}
	return c.JSON(body)
}
