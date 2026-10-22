package store

import (
	"context"
)

func (s *Store) ListFavorites(ctx context.Context, memberID int64) ([]Movie, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT m.id, m.title, m.title_th,
		        (SELECT coalesce(g.name_th, g.name_en) FROM movie_genres mg
		           JOIN genres g ON g.id = mg.genre_id
		          WHERE mg.movie_id = m.id AND mg.position = 0),
		        extract(year FROM m.release_date)::int,
		        round(m.vote_average, 1)::float8,
		        'https://image.tmdb.org/t/p/w500' || m.poster_path
		 FROM favorites f
		 JOIN movies m ON m.id = f.movie_id
		 WHERE f.member_id = $1
		 ORDER BY f.added_at DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Movie{}
	for rows.Next() {
		var m Movie
		if err := rows.Scan(&m.ID, &m.Title, &m.TitleTh, &m.Genre, &m.Year, &m.Rating, &m.Poster); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) AddFavorite(ctx context.Context, memberID, movieID int64) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO favorites (member_id, movie_id)
		 SELECT $1, id FROM movies WHERE id = $2
		 ON CONFLICT (member_id, movie_id) DO NOTHING`, memberID, movieID)
	if err != nil {
		return err
	}
	return s.ensureMovieExists(ctx, movieID)
}

func (s *Store) RemoveFavorite(ctx context.Context, memberID, movieID int64) error {
	if err := s.ensureMovieExists(ctx, movieID); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM favorites WHERE member_id = $1 AND movie_id = $2`, memberID, movieID)
	return err
}

func (s *Store) ensureMovieExists(ctx context.Context, movieID int64) error {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM movies WHERE id = $1)`, movieID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}
