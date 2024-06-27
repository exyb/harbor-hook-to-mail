package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exyb/harbor-hook-to-mail/routes"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWebhookHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.Default()
	routes.SetupRouter(r)

	testRequest := routes.WebhookRequest{
		Type:     "PUSH_ARTIFACT",
		OccurAt:  1716714783,
		Operator: "admin",
		EventData: struct {
			Resources []struct {
				Digest      string "json:\"digest\""
				Tag         string "json:\"tag\""
				ResourceURL string "json:\"resource_url\""
			} "json:\"resources\""
			Repository struct {
				DateCreated  int64  "json:\"date_created\""
				Name         string "json:\"name\""
				Namespace    string "json:\"namespace\""
				RepoFullName string "json:\"repo_full_name\""
				RepoType     string "json:\"repo_type\""
			} "json:\"repository\""
		}{
			Resources: []struct {
				Digest      string "json:\"digest\""
				Tag         string "json:\"tag\""
				ResourceURL string "json:\"resource_url\""
			}{
				{
					Digest:      "sha256:551816281922709f43d1d1ce3e10b8e60d07ad1ad4750454b2aae97b97ac2f86",
					Tag:         "test_20240526171000",
					ResourceURL: "harbor.example.com/build-hook/test-app:p0_20240526171000",
				},
			},
			Repository: struct {
				DateCreated  int64  "json:\"date_created\""
				Name         string "json:\"name\""
				Namespace    string "json:\"namespace\""
				RepoFullName string "json:\"repo_full_name\""
				RepoType     string "json:\"repo_type\""
			}{
				DateCreated:  1716714783,
				Name:         "test-app",
				Namespace:    "build-hook",
				RepoFullName: "build-hook/test-app",
				RepoType:     "public",
			},
		},
	}

	body, _ := json.Marshal(testRequest)

	req, _ := http.NewRequest(http.MethodPost, "/hook/app", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status": "success"}`, w.Body.String())
}
