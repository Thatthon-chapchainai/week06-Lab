# MovieHub API

## Run locally

From the repository root, start PostgreSQL:

```sh
docker compose up -d db
```

In `moviehub/.env`, set `DATABASE_URL` to match the local PostgreSQL database in
`docker-compose.yml`. Set `ALLOWED_ORIGINS` to the frontend origin. Provide a
`JWT_SECRET` of at least 32 characters, or a local `.jwt_secret` file.

For an existing database created before favorites was introduced, apply the
additive migration from the `moviehub` directory:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_favorites.sql
```

`psql` reads `DATABASE_URL` from the shell environment; Go loads `.env`
automatically. For a new database, initialize it with `schema.sql` and `seed.sql`
instead. Do not reapply `schema.sql` to an existing database.

Start the API from the `moviehub` directory:

```sh
go run .
```

The default port is 8080.

## Wishlist API

These routes match the wishlist button in the deployed frontend. Each requires
`Authorization: Bearer <token>` from `POST /api/auth/login`.

| Method | Route | JSON response |
| --- | --- | --- |
| GET | `/api/me/wishlist` | `{"items":[...]}` (movie objects; `[]` when empty) |
| PUT | `/api/me/wishlist/{id}` | `{"wishlist":true}` |
| DELETE | `/api/me/wishlist/{id}` | `{"wishlist":false}` |

Adding or removing the same movie repeatedly is safe. The authenticated member's
ID comes from the token. Invalid IDs return JSON with status 400; unknown movies
return JSON with status 404; missing or invalid tokens return JSON with status 401.
Wishlists and favorites are stored separately.

## Validation

```sh
go test ./...
```

For integration tests, set `MOVIEHUB_TEST_DATABASE_URL` to a local PostgreSQL
connection string and run the same command. The database user must be allowed to
create schemas. Tests create and drop a temporary schema and leave existing
application data untouched.

## Deploy

The Vercel frontend at `https://moviehub-670710719.vercel.app` calls
`https://moviehub-2s68.onrender.com`; restarting the local API does not update it.
Deploy this backend revision to that Render service. Keep the service's existing
`DATABASE_URL` and `JWT_SECRET`, and include the Vercel origin in `ALLOWED_ORIGINS`.
The wishlist routes use the existing `wishlist_items` table. Apply the favorites
migration only if the separate `favorites` table is missing.

After deployment, `GET /api/me/wishlist` without a token should return JSON with
status 401, rather than a plain-text 404. Log in on the frontend and confirm that
adding a movie survives a page refresh and that removing it clears the wishlist.
