package handlers

import (
	"database/sql"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/CodeToole/waitaminutedigital_go/internal/db"
	"github.com/CodeToole/waitaminutedigital_go/internal/models"
	"github.com/labstack/echo/v4"
)

func SubmitContact(siteURL string, database *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		var values models.ContactSubmission
		if err := c.Bind(&values); err != nil {
			return renderContact(c, siteURL, values, map[string]string{"form": "Please check the submitted form and try again."}, false, http.StatusBadRequest)
		}

		values.Name = strings.TrimSpace(values.Name)
		values.Email = strings.TrimSpace(values.Email)
		values.Subject = strings.TrimSpace(values.Subject)
		values.Message = strings.TrimSpace(values.Message)

		// Quietly accept automated submissions without persisting them.
		if strings.TrimSpace(values.Website) != "" {
			return renderContact(c, siteURL, models.ContactSubmission{}, nil, true, http.StatusOK)
		}

		fieldErrors := validateContact(values)
		if len(fieldErrors) > 0 {
			return renderContact(c, siteURL, values, fieldErrors, false, http.StatusUnprocessableEntity)
		}

		inquiry := models.Inquiry{
			Name:    values.Name,
			Email:   values.Email,
			Subject: values.Subject,
			Message: values.Message,
		}
		if err := db.CreateInquiry(c.Request().Context(), database, inquiry); err != nil {
			c.Logger().Error(err)
			return renderContact(c, siteURL, values, map[string]string{"form": "We could not send your message. Please try again."}, false, http.StatusInternalServerError)
		}
		return renderContact(c, siteURL, models.ContactSubmission{}, nil, true, http.StatusOK)
	}
}

func validateContact(values models.ContactSubmission) map[string]string {
	fieldErrors := make(map[string]string)
	if values.Name == "" {
		fieldErrors["name"] = "Please enter your name."
	} else if utf8.RuneCountInString(values.Name) > 100 {
		fieldErrors["name"] = "Name must be 100 characters or fewer."
	}

	parsedEmail, err := mail.ParseAddress(values.Email)
	if err != nil || parsedEmail.Address != values.Email || len(values.Email) > 254 {
		fieldErrors["email"] = "Enter a valid email address."
	}

	if !models.IsContactSubject(values.Subject) {
		fieldErrors["subject"] = "Choose one of the listed subjects."
	}
	messageLength := utf8.RuneCountInString(values.Message)
	if messageLength < 10 {
		fieldErrors["message"] = "Please include a little more detail (at least 10 characters)."
	} else if messageLength > 5000 {
		fieldErrors["message"] = "Message must be 5,000 characters or fewer."
	}
	return fieldErrors
}
