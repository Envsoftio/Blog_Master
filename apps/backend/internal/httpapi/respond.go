package httpapi

import "github.com/gofiber/fiber/v2"

const problemMediaType = "application/problem+json"

func writeJSON(c *fiber.Ctx, status int, value any) error {
	return c.Status(status).JSON(value)
}

func problem(c *fiber.Ctx, status int, title string, detail string) error {
	return c.Status(status).JSON(Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	}, problemMediaType)
}

func notImplemented(c *fiber.Ctx, area string) error {
	return problem(c, fiber.StatusNotImplemented, "Not implemented", area+" is scaffolded and awaiting workflow implementation")
}
