package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	profileapp "cixing/internal/modules/profile/application"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	"cixing/internal/transport/http/server"
	v1 "cixing/internal/transport/http/v1"
)

func notificationSettingsRouter(pool *pgxpool.Pool, users map[string]string) http.Handler {
	return server.NewRouter(server.Options{
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		GinMode:             gin.TestMode,
		AccessTokenVerifier: testAccessTokenVerifier{users: users},
		V1: &v1.Handler{
			Profile: profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), nil),
		},
	})
}

// Run against the real OpenAPI/auth middleware even without a test database.
func TestNotificationSettingsHTTPValidation(t *testing.T) {
	router := notificationSettingsRouter(nil, map[string]string{"token": uuid.NewString()})
	for _, tc := range []struct {
		name   string
		method string
		path   string
		token  string
		body   string
		status int
	}{
		{"unauthorized read", http.MethodGet, "/v1/me/settings", "", "", http.StatusUnauthorized},
		{"unauthorized update", http.MethodPatch, "/v1/me/settings/notifications", "", `{"creation_reminder_enabled":true}`, http.StatusUnauthorized},
		{"invalid token", http.MethodPatch, "/v1/me/settings/notifications", "invalid", `{"reaction_enabled":false}`, http.StatusUnauthorized},
		{"empty object", http.MethodPatch, "/v1/me/settings/notifications", "token", `{}`, http.StatusBadRequest},
		{"missing body", http.MethodPatch, "/v1/me/settings/notifications", "token", "", http.StatusBadRequest},
		{"null body", http.MethodPatch, "/v1/me/settings/notifications", "token", `null`, http.StatusBadRequest},
		{"null reminder", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"creation_reminder_enabled":null}`, http.StatusBadRequest},
		{"null reaction with reminder", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"reaction_enabled":null,"creation_reminder_enabled":false}`, http.StatusBadRequest},
		{"string reminder", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"creation_reminder_enabled":"false"}`, http.StatusBadRequest},
		{"numeric reaction", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"reaction_enabled":0}`, http.StatusBadRequest},
		{"unknown field", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"creation_enabled":true}`, http.StatusBadRequest},
		{"unexpected user id", http.MethodPatch, "/v1/me/settings/notifications", "token", `{"reaction_enabled":false,"user_id":"another-user"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, router, tc.method, tc.path, tc.token, []byte(tc.body))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestNotificationSettingsHTTPPersistence(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)
	userID, otherID := uuid.New(), uuid.New()
	seedUser(t, ctx, pool, userID, "reminder@example.com", "Reminder")
	seedUser(t, ctx, pool, otherID, "other-reminder@example.com", "Other")
	// Exercise the schema defaults rather than seedUser's explicit boolean values.
	if _, err := pool.Exec(ctx, `UPDATE user_settings SET reaction_notification_enabled = DEFAULT, creation_reminder_enabled = DEFAULT WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	router := notificationSettingsRouter(pool, map[string]string{
		"token": userID.String(), "other": otherID.String(), "missing": uuid.NewString(),
	})
	assertNotificationSettings(t, performRequest(t, router, http.MethodGet, "/v1/me/settings", "token", nil), true, true)

	for _, tc := range []struct {
		name     string
		body     string
		reaction bool
		creation bool
	}{
		{"old client disables reactions", `{"reaction_enabled":false}`, false, true},
		{"disable reminder independently", `{"creation_reminder_enabled":false}`, false, false},
		{"old client enables reactions", `{"reaction_enabled":true}`, true, false},
		{"enable reminder independently", `{"creation_reminder_enabled":true}`, true, true},
		{"disable both", `{"reaction_enabled":false,"creation_reminder_enabled":false}`, false, false},
		{"repeat update", `{"creation_reminder_enabled":false}`, false, false},
		{"enable both", `{"reaction_enabled":true,"creation_reminder_enabled":true}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, router, http.MethodPatch, "/v1/me/settings/notifications", "token", []byte(tc.body))
			assertNotificationSettings(t, rec, tc.reaction, tc.creation)
			// Read in a separate request, and verify the database values directly.
			assertNotificationSettings(t, performRequest(t, router, http.MethodGet, "/v1/me/settings", "token", nil), tc.reaction, tc.creation)
			var reaction, creation bool
			if err := pool.QueryRow(ctx, `SELECT reaction_notification_enabled, creation_reminder_enabled FROM user_settings WHERE user_id = $1`, userID).Scan(&reaction, &creation); err != nil {
				t.Fatal(err)
			}
			if reaction != tc.reaction || creation != tc.creation {
				t.Fatalf("persisted settings = (%t, %t), want (%t, %t)", reaction, creation, tc.reaction, tc.creation)
			}
			assertNotificationSettings(t, performRequest(t, router, http.MethodGet, "/v1/me/settings", "other", nil), true, true)
		})
	}

	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		path := "/v1/me/settings"
		var body []byte
		if method == http.MethodPatch {
			path += "/notifications"
			body = []byte(`{"creation_reminder_enabled":false}`)
		}
		rec := performRequest(t, router, method, path, "missing", body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s missing settings: status = %d, want 404; body=%s", method, rec.Code, rec.Body.String())
		}
	}
}

func TestNotificationSettingsConcurrentIndependentUpdates(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)
	userID := uuid.New()
	seedUser(t, ctx, pool, userID, "concurrent-reminder@example.com", "Concurrent")
	svc := profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), nil)
	// Concurrent patches to different fields must never overwrite each other.
	for attempt := 0; attempt < 10; attempt++ {
		if _, err := svc.UpdateNotificationSettings(ctx, userID, profileapp.NotificationSettingsPatch{
			ReactionEnabled: boolPtr(true), CreationReminderEnabled: boolPtr(true),
		}); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, patch := range []profileapp.NotificationSettingsPatch{
			{ReactionEnabled: boolPtr(false)}, {CreationReminderEnabled: boolPtr(false)},
		} {
			go func() {
				<-start
				_, err := svc.UpdateNotificationSettings(ctx, userID, patch)
				results <- err
			}()
		}
		close(start)
		for range 2 {
			if err := <-results; err != nil {
				t.Error(err)
			}
		}
		settings, err := svc.GetSettings(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		if settings.Notifications.ReactionEnabled || settings.Notifications.CreationReminderEnabled {
			t.Fatalf("attempt %d lost an update: %+v", attempt, settings.Notifications)
		}
	}
}

func assertNotificationSettings(t *testing.T, rec *httptest.ResponseRecorder, reaction, creation bool) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Profile       json.RawMessage `json:"profile"`
		Notifications map[string]bool `json:"notifications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if len(body.Profile) == 0 || string(body.Profile) == "null" {
		t.Fatal("settings response is missing the profile")
	}
	for field, want := range map[string]bool{"reaction_enabled": reaction, "creation_reminder_enabled": creation} {
		if got, present := body.Notifications[field]; !present || got != want {
			t.Fatalf("notifications.%s = %t (present=%t), want %t", field, got, present, want)
		}
	}
}
