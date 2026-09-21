package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/DimwitLabs/tuck/internal/store"
)

const maxItemCiphertext = 1 << 20

var itemKinds = map[string]bool{"credential": true, "host": true, "file": true, "manifest": true}

func itemID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return "", fail(http.StatusBadRequest, "item ids are lowercase UUIDs")
	}
	return id, nil
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	items, err := s.store.ListItems(r.Context(), a.session.UserID)
	if err != nil {
		return err
	}
	if items == nil {
		items = []store.Item{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (s *Server) putItem(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	id, err := itemID(r)
	if err != nil {
		return err
	}
	var body struct {
		Kind       string  `json:"kind"`
		Nonce      string  `json:"nonce"`
		Ciphertext string  `json:"ciphertext"`
		Replaces   *string `json:"replaces"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if !itemKinds[body.Kind] {
		return fail(http.StatusBadRequest, "unknown item kind")
	}
	if !s.cfg.Features.allows(body.Kind) {
		return s.featureOff()
	}
	nonce, err := b64("nonce", body.Nonce, 12)
	if err != nil {
		return err
	}
	ciphertext, err := b64("ciphertext", body.Ciphertext, 0)
	if err != nil {
		return err
	}
	if len(ciphertext) < 17 {
		return fail(http.StatusBadRequest, "that is not an encrypted item")
	}
	if len(ciphertext) > maxItemCiphertext {
		return fail(http.StatusRequestEntityTooLarge, "items are limited to 1 mb; store large content as a file")
	}
	var expect []byte
	if body.Replaces != nil {
		expect = []byte{}
		if *body.Replaces != "" {
			if expect, err = b64("replaces", *body.Replaces, 32); err != nil {
				return err
			}
		}
	}
	err = s.store.PutItem(r.Context(), a.session.UserID, store.Item{ID: id, Kind: body.Kind, Nonce: nonce, Ciphertext: ciphertext}, s.cfg.Quota, expect)
	if errors.Is(err, store.ErrStale) {
		return fail(http.StatusPreconditionFailed, "this was changed in another tab or device since you opened it; lock and unlock to get the newest copy, then try again")
	}
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusConflict, "an item's kind cannot change")
	}
	if errors.Is(err, store.ErrQuota) {
		return s.vaultFull()
	}
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	id, err := itemID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteItem(r.Context(), a.session.UserID, id); errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusNotFound, "no such item")
	} else if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

const fileOverhead = 12 + 16

func (s *Server) putFile(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	id, err := itemID(r)
	if err != nil {
		return err
	}
	if !s.cfg.Features.allows("file") {
		return s.featureOff()
	}
	blob, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.cfg.MaxFileBytes+fileOverhead))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fail(http.StatusRequestEntityTooLarge, "files are limited to %d mb", s.cfg.MaxFileBytes>>20)
	}
	if err != nil {
		return err
	}
	if len(blob) < fileOverhead {
		return fail(http.StatusBadRequest, "that is not an encrypted file")
	}
	switch err := s.store.PutFile(r.Context(), a.session.UserID, id, blob, s.cfg.Quota); {
	case errors.Is(err, store.ErrNotFound):
		return fail(http.StatusNotFound, "save the file's item before uploading its contents")
	case errors.Is(err, store.ErrQuota):
		return s.vaultFull()
	case err != nil:
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	id, err := itemID(r)
	if err != nil {
		return err
	}
	blob, err := s.store.File(r.Context(), a.session.UserID, id)
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusNotFound, "no such file")
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = w.Write(blob)
	return err
}

func (s *Server) vaultFull() error {
	return fail(http.StatusInsufficientStorage, "your vault is full: %d items or %d mb at most; delete something first",
		s.cfg.Quota.Items, s.cfg.Quota.Bytes>>20)
}

func (s *Server) featureOff() error {
	return fail(http.StatusForbidden, "this tuck is set up for %s only", map[Features]string{FeaturesSSH: "ssh", FeaturesFiles: "files"}[s.cfg.Features])
}
