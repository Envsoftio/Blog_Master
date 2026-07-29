package httpapi

import "github.com/gofiber/fiber/v2"

func writeJSON(c *fiber.Ctx, status int, value any) error {
	return c.Status(status).JSON(value)
}

func problem(c *fiber.Ctx, status int, title string, detail string) error {
	return writeJSON(c, status, Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

func notImplemented(c *fiber.Ctx, area string) error {
	return problem(c, fiber.StatusNotImplemented, "Not implemented", area+" is scaffolded and awaiting workflow implementation")
}
