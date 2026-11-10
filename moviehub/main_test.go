package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"moviehub/auth"
	"moviehub/handler"
	"moviehub/store"
)

type apiResponse struct {
	Items    []store.Movie `json:"items"`
	Wishlist *bool         `json:"wishlist"`
	Favorite *bool         `json:"favorite"`
	Token    string        `json:"token"`
	Error    struct {
		Code string `json:"code"`
	} `json:"error"`
}

func requestJSON(t *testing.T, api http.Handler, method, path, token, body string, status int) apiResponse {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s: status = %d, want %d; body: %s", method, path, w.Code, status, w.Body)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("%s %s: expected JSON, got %q; body: %s", method, path, w.Header().Get("Content-Type"), w.Body)
	}
	var response apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("%s %s: invalid JSON: %v", method, path, err)
	}
	return response
}

func TestWishlistValidation(t *testing.T) {
	tickets := &auth.Issuer{Secret: []byte("test-secret-with-at-least-32-characters"), TTL: time.Hour}
	api := newMux(&handler.Handler{Tickets: tickets}, tickets)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/me/wishlist"},
		{http.MethodPut, "/api/me/wishlist/969681"},
		{http.MethodDelete, "/api/me/wishlist/969681"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			for _, token := range []string{"", "invalid-token"} {
				got := requestJSON(t, api, route.method, route.path, token, "", http.StatusUnauthorized)
				if got.Error.Code != "NO_TOKEN" && got.Error.Code != "BAD_TOKEN" {
					t.Fatalf("unexpected error: %q", got.Error.Code)
				}
			}
		})
	}
	token, err := tickets.Issue(1, "member")
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		for _, id := range []string{"abc", "0", "-1", "9223372036854775808"} {
			got := requestJSON(t, api, method, "/api/me/wishlist/"+id, token, "", http.StatusBadRequest)
			if got.Error.Code != "BAD_ID" {
				t.Fatalf("unexpected error: %q", got.Error.Code)
			}
		}
	}
}

func TestWishlistAndFavoritesIntegration(t *testing.T) {
	databaseURL := os.Getenv("MOVIEHUB_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MOVIEHUB_TEST_DATABASE_URL to run PostgreSQL integration tests in a temporary schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	schema := pgx.Identifier{fmt.Sprintf("wishlist_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("remove temporary schema: %v", err)
		}
	}()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ddl, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO movies (id, title, original_title, original_language)
		VALUES (969681, 'Wishlist test movie', 'Wishlist test movie', 'en')`); err != nil {
		t.Fatal(err)
	}
	// The upgrade must create the missing favorites table without clearing wishlists.
	if _, err := pool.Exec(ctx, "DROP TABLE favorites"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("migrations/001_favorites.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	tickets := &auth.Issuer{Secret: []byte("test-secret-with-at-least-32-characters"), TTL: time.Hour}
	api := newMux(&handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}, tickets)
	registerAndLogin := func(email string) string {
		t.Helper()
		body := fmt.Sprintf(`{"email":%q,"password":"test-password-123","displayName":"Test member"}`, email)
		requestJSON(t, api, http.MethodPost, "/api/auth/register", "", body, http.StatusCreated)
		response := requestJSON(t, api, http.MethodPost, "/api/auth/login", "", body, http.StatusOK)
		if response.Token == "" {
			t.Fatal("login returned no token")
		}
		return response.Token
	}
	first := registerAndLogin("first@example.test")
	second := registerAndLogin("second@example.test")
	assertItems := func(token string, count int) {
		t.Helper()
		response := requestJSON(t, api, http.MethodGet, "/api/me/wishlist", token, "", http.StatusOK)
		if response.Items == nil || len(response.Items) != count {
			t.Fatalf("wishlist should contain %d items in an array, got %v", count, response.Items)
		}
		if count == 1 && response.Items[0].ID != 969681 {
			t.Fatalf("unexpected wishlist movie: %d", response.Items[0].ID)
		}
	}
	assertItems(first, 0)
	for range 2 {
		response := requestJSON(t, api, http.MethodPut, "/api/me/wishlist/969681", first, "", http.StatusOK)
		if response.Wishlist == nil || !*response.Wishlist {
			t.Fatal("adding a movie should return wishlist: true")
		}
	}
	assertItems(first, 1)
	assertItems(second, 0)
	requestJSON(t, api, http.MethodDelete, "/api/me/wishlist/969681", second, "", http.StatusOK)
	assertItems(first, 1)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		response := requestJSON(t, api, method, "/api/me/wishlist/9223372036854775807", first, "", http.StatusNotFound)
		if response.Error.Code != "MOVIE_NOT_FOUND" {
			t.Fatalf("unexpected error: %q", response.Error.Code)
		}
	}
	response := requestJSON(t, api, http.MethodPut, "/api/movies/969681/favorite", first, "", http.StatusOK)
	if response.Favorite == nil || !*response.Favorite {
		t.Fatal("adding a favorite should return favorite: true")
	}
	assertItems(first, 1)
	for range 2 {
		response := requestJSON(t, api, http.MethodDelete, "/api/me/wishlist/969681", first, "", http.StatusOK)
		if response.Wishlist == nil || *response.Wishlist {
			t.Fatal("removing a movie should return wishlist: false")
		}
	}
	assertItems(first, 0)
	favorites := requestJSON(t, api, http.MethodGet, "/api/me/favorites", first, "", http.StatusOK)
	if len(favorites.Items) != 1 || favorites.Items[0].ID != 969681 {
		t.Fatal("removing a wishlist movie must preserve favorites")
	}
	requestJSON(t, api, http.MethodDelete, "/api/movies/969681/favorite", first, "", http.StatusOK)
}
