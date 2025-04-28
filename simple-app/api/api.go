package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-app/internal/example"
)

// Api connects HTTP routes to the service logic.
type Api struct {
	service example.Service
}

// New creates a new Api instance.
func New(service example.Service) *Api {
	return &Api{service: service}
}

// AddMessage handles POST /add requests. It reads JSON, adds the message using the service, and sends back a response.
func AddMessage(a *Api) gin.HandlerFunc {
	return func(c *gin.Context) {
		var exampleEntity example.Entity
		if err := c.ShouldBindJSON(&exampleEntity); err != nil {
			c.JSON(http.StatusBadRequest, "Wrong json format")
			return
		}
		err := a.service.AddMessage(c, &exampleEntity)
		if err != nil {
			c.JSON(http.StatusInternalServerError, "Failed to add message")
			return
		}
		c.JSON(http.StatusOK, "Message added successfully")
	}
}
