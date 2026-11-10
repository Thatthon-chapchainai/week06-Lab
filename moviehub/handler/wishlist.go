package handler

import (
	"errors"
	"net/http"

	"moviehub/mw"
	"moviehub/store"
)

func (h *Handler) ListWishlist(w http.ResponseWriter, r *http.Request) {
	memberID, ok := mw.MemberID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	items, err := h.Store.ListWishlist(r.Context(), memberID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddWishlist(w http.ResponseWriter, r *http.Request) {
	h.setWishlist(w, r, true)
}

func (h *Handler) RemoveWishlist(w http.ResponseWriter, r *http.Request) {
	h.setWishlist(w, r, false)
}

func (h *Handler) setWishlist(w http.ResponseWriter, r *http.Request, add bool) {
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
		err = h.Store.AddWishlist(r.Context(), memberID, movieID)
	} else {
		err = h.Store.RemoveWishlist(r.Context(), memberID, movieID)
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"wishlist": add})
}
