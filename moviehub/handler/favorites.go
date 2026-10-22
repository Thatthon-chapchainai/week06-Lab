package handler

import (
	"errors"
	"net/http"

	"moviehub/mw"
	"moviehub/store"
)

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	memberID, ok := mw.MemberID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	items, err := h.Store.ListFavorites(r.Context(), memberID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	h.setFavorite(w, r, true)
}

func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	h.setFavorite(w, r, false)
}

func (h *Handler) setFavorite(w http.ResponseWriter, r *http.Request, add bool) {
	movieID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
		return
	}
	memberID, ok := mw.MemberID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}

	var err error
	if add {
		err = h.Store.AddFavorite(r.Context(), memberID, movieID)
	} else {
		err = h.Store.RemoveFavorite(r.Context(), memberID, movieID)
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorite": add})
}
