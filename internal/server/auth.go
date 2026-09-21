package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/DimwitLabs/tuck/internal/store"
)

var defaultKDF = store.KDF{Algorithm: "argon2id", Iterations: 3, MemoryKiB: 65536, Parallelism: 1}

func sha(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	if err := s.store.Ping(r.Context()); err != nil {
		return fail(http.StatusServiceUnavailable, "database unreachable")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (s *Server) signupOpen(r *http.Request) (bool, error) {
	n, err := s.store.CountUsers(r.Context())
	return n == 0, err
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) error {
	if !s.gatePassed(r) {
		writeJSON(w, http.StatusOK, map[string]any{"gate": true, "word": s.cfg.GateWord})
		return nil
	}
	open, err := s.signupOpen(r)
	if err != nil {
		return err
	}
	body := map[string]any{
		"gate": false, "signupOpen": open, "features": s.cfg.Features, "maxFileBytes": s.cfg.MaxFileBytes, "session": nil,
	}
	if a, err := s.session(r); err == nil {
		u, err := s.store.UserByID(r.Context(), a.session.UserID)
		if err != nil {
			return err
		}
		body["session"] = map[string]any{"username": u.Username, "unlocked": a.session.UnlockStage == 3}
	}
	writeJSON(w, http.StatusOK, body)
	return nil
}

func (s *Server) prelogin(w http.ResponseWriter, r *http.Request) error {
	if !s.gatePassed(r) {
		return errGateClosed
	}
	username := normalizeUsername(r.URL.Query().Get("username"))
	if !s.loginByIP.allow("pre:" + limiterKey(s.clientIP(r))) {
		return fail(http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
	}
	u, err := s.store.UserByName(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		mac := hmac.New(sha256.New, s.cfg.Secret)
		mac.Write([]byte("decoy:" + username))
		writeJSON(w, http.StatusOK, map[string]any{"kdf": defaultKDF, "kdfSalt": mac.Sum(nil)[:16]})
		return nil
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"kdf": u.KDF, "kdfSalt": u.KDFSalt})
	return nil
}

type stepJSON struct {
	Salt               string `json:"salt"`
	QuestionNonce      string `json:"questionNonce"`
	QuestionCiphertext string `json:"questionCiphertext"`
	Proof              string `json:"proof"`
}

type enrollmentJSON struct {
	KDF          store.KDF  `json:"kdf"`
	KDFSalt      string     `json:"kdfSalt"`
	AuthKey      string     `json:"authKey"`
	Steps        []stepJSON `json:"steps"`
	WrappedKey   string     `json:"wrappedKey"`
	WrappedNonce string     `json:"wrappedNonce"`
}

func (e enrollmentJSON) parse() (store.Enrollment, error) {
	var out store.Enrollment
	k := e.KDF
	if k.Algorithm != "argon2id" || k.Iterations < 2 || k.Iterations > 10 ||
		k.MemoryKiB < 19456 || k.MemoryKiB > 1<<20 || k.Parallelism < 1 || k.Parallelism > 4 {
		return out, fail(http.StatusBadRequest, "key derivation settings are outside the allowed range")
	}
	out.KDF = k
	var err error
	if out.KDFSalt, err = b64("kdfSalt", e.KDFSalt, 16); err != nil {
		return out, err
	}
	authKey, err := b64("authKey", e.AuthKey, 32)
	if err != nil {
		return out, err
	}
	out.AuthHash = sha(authKey)
	if out.WrappedKey, err = b64("wrappedKey", e.WrappedKey, 48); err != nil {
		return out, err
	}
	if out.WrappedNonce, err = b64("wrappedNonce", e.WrappedNonce, 12); err != nil {
		return out, err
	}
	if len(e.Steps) != 3 {
		return out, fail(http.StatusBadRequest, "exactly three questions are required")
	}
	for i, st := range e.Steps {
		var step store.Step
		if step.Salt, err = b64("salt", st.Salt, 16); err != nil {
			return out, err
		}
		if step.QuestionNonce, err = b64("questionNonce", st.QuestionNonce, 12); err != nil {
			return out, err
		}
		if step.QuestionCiphertext, err = b64("questionCiphertext", st.QuestionCiphertext, 0); err != nil {
			return out, err
		}
		if len(step.QuestionCiphertext) < 17 || len(step.QuestionCiphertext) > 1024 {
			return out, fail(http.StatusBadRequest, "each question must be between 1 and about 1000 characters")
		}
		proof, err := b64("proof", st.Proof, 32)
		if err != nil {
			return out, err
		}
		step.ProofHash = sha(proof)
		out.Steps[i] = step
	}
	return out, nil
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) error {
	if !s.signupByIP.allow(limiterKey(s.clientIP(r))) {
		return fail(http.StatusTooManyRequests, "too many sign-ups from here; try again later")
	}
	var body struct {
		Username   string         `json:"username"`
		Enrollment enrollmentJSON `json:"enrollment"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if !s.gatePassed(r) {
		return errGateClosed
	}
	username := normalizeUsername(body.Username)
	if !usernamePattern.MatchString(username) {
		return fail(http.StatusBadRequest, "usernames are 3 to 64 characters: letters, digits, dot, dash or underscore")
	}
	e, err := body.Enrollment.parse()
	if err != nil {
		return err
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	switch err := s.store.CreateUser(r.Context(), id, username, e); {
	case errors.Is(err, store.ErrSignupClosed):
		return fail(http.StatusForbidden, "this tuck already has its one account")
	case err != nil:
		return err
	}
	if err := s.startSession(r.Context(), w, id, 3); err != nil {
		return err
	}
	s.markDevice(w, id)
	s.audit(r, "signup")
	writeJSON(w, http.StatusCreated, map[string]string{"username": username})
	return nil
}

var decoyAuthHash = sha([]byte("tuck-decoy-auth"))

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Username string `json:"username"`
		AuthKey  string `json:"authKey"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if !s.gatePassed(r) {
		return errGateClosed
	}
	if !s.loginByIP.allow(limiterKey(s.clientIP(r))) {
		return fail(http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
	}
	username := normalizeUsername(body.Username)
	authKey, err := b64("authKey", body.AuthKey, 32)
	if err != nil {
		return err
	}
	if !usernamePattern.MatchString(username) {
		s.audit(r, "login_failed")
		return fail(http.StatusUnauthorized, "wrong username or password")
	}
	u, err := s.store.UserByName(r.Context(), username)
	expected := decoyAuthHash
	if err == nil {
		expected = u.AuthHash
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if (u == nil || !s.knownDevice(r, u.ID)) && !s.loginByUser.allow(username) {
		return fail(http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
	}
	if subtle.ConstantTimeCompare(sha(authKey), expected) != 1 || u == nil {
		s.audit(r, "login_failed")
		return fail(http.StatusUnauthorized, "wrong username or password")
	}
	if old, err := s.session(r); err == nil {
		if err := s.store.DeleteSession(r.Context(), old.tokenHash); err != nil {
			return err
		}
	}
	if err := s.startSession(r.Context(), w, u.ID, 0); err != nil {
		return err
	}
	s.markDevice(w, u.ID)
	s.audit(r, "login")
	writeJSON(w, http.StatusOK, map[string]string{"username": u.Username})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if a, err := s.session(r); err == nil {
		if err := s.store.DeleteSession(r.Context(), a.tokenHash); err != nil {
			return err
		}
		s.audit(r, "logout")
	}
	s.setCookie(w, sessionCookie, "", -1)
	s.setCookie(w, gateCookie, "", -1)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func stepPublic(st *store.Step) map[string][]byte {
	return map[string][]byte{"salt": st.Salt, "questionNonce": st.QuestionNonce, "questionCiphertext": st.QuestionCiphertext}
}

func frozen(w http.ResponseWriter) error {
	writeJSON(w, http.StatusLocked, map[string]any{
		"error":  "unlocking is frozen after too many wrong answers; whoever runs this tuck has to lift it in the database",
		"frozen": true,
	})
	return nil
}

func lockedOut(w http.ResponseWriter, until time.Time) error {
	secs := int(time.Until(until).Seconds()) + 1
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":      "too many wrong answers; try again later",
		"retryAfter": secs,
	})
	return nil
}

func (s *Server) unlockStart(w http.ResponseWriter, r *http.Request) error {
	a, err := s.session(r)
	if err != nil {
		return err
	}
	u, err := s.store.UserByID(r.Context(), a.session.UserID)
	if err != nil {
		return err
	}
	if u.UnlockFrozen {
		return frozen(w)
	}
	if u.UnlockLockedUntil != nil && time.Now().Before(*u.UnlockLockedUntil) {
		return lockedOut(w, *u.UnlockLockedUntil)
	}
	if err := s.store.SetUnlockStage(r.Context(), a.tokenHash, 0); err != nil {
		return err
	}
	st, err := s.store.Step(r.Context(), u.ID, 1)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"step": 1, "question": stepPublic(st)})
	return nil
}

func (s *Server) unlockAnswer(w http.ResponseWriter, r *http.Request) error {
	a, err := s.session(r)
	if err != nil {
		return err
	}
	var body struct {
		Step  int    `json:"step"`
		Proof string `json:"proof"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if !s.answerByIP.allow(limiterKey(s.clientIP(r))) {
		return fail(http.StatusTooManyRequests, "too many attempts; try again in a few minutes")
	}
	proof, err := b64("proof", body.Proof, 32)
	if err != nil {
		return err
	}
	if body.Step != a.session.UnlockStage+1 || body.Step < 1 || body.Step > 3 {
		return fail(http.StatusConflict, "questions must be answered in order; start again from the first")
	}
	userID := a.session.UserID
	res, err := s.store.CheckAnswer(r.Context(), userID, body.Step, sha(proof), s.cfg.Lockout)
	if err != nil {
		return err
	}
	if res.Frozen {
		s.audit(r, "unlock_frozen", "step", body.Step)
		return frozen(w)
	}
	if res.LockedUntil != nil {
		s.audit(r, "unlock_locked_out", "step", body.Step, "until", res.LockedUntil.UTC())
		return lockedOut(w, *res.LockedUntil)
	}
	if !res.OK {
		s.audit(r, "unlock_wrong_answer", "step", body.Step, "attemptsLeft", res.AttemptsLeft)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":              "that's not the answer",
			"attemptsBeforeLock": res.AttemptsLeft,
			"freezesNext":        res.FreezesNext,
		})
		return nil
	}

	if body.Step < 3 {
		ok, err := s.store.AdvanceUnlock(r.Context(), a.tokenHash, body.Step-1, body.Step)
		if err != nil {
			return err
		}
		if !ok {
			return fail(http.StatusConflict, "questions must be answered in order; start again from the first")
		}
		next, err := s.store.Step(r.Context(), userID, body.Step+1)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"step": body.Step + 1, "question": stepPublic(next)})
		return nil
	}
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	ok, err := s.store.FinishUnlock(r.Context(), a.tokenHash, hash)
	if err != nil {
		return err
	}
	if !ok {
		return fail(http.StatusConflict, "questions must be answered in order; start again from the first")
	}
	s.setSessionCookie(w, token, a.session.ExpiresAt)
	if err := s.store.ResetUnlockFailures(r.Context(), userID); err != nil {
		return err
	}
	s.audit(r, "unlock")
	u, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"wrappedKey": u.WrappedKey, "wrappedNonce": u.WrappedNonce})
	return nil
}

func (s *Server) lock(w http.ResponseWriter, r *http.Request) error {
	a, err := s.session(r)
	if err != nil {
		return err
	}
	if err := s.store.SetUnlockStage(r.Context(), a.tokenHash, 0); err != nil {
		return err
	}
	s.audit(r, "lock")
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) rekey(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	var body struct {
		CurrentAuthKey string         `json:"currentAuthKey"`
		Enrollment     enrollmentJSON `json:"enrollment"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	current, err := b64("currentAuthKey", body.CurrentAuthKey, 32)
	if err != nil {
		return err
	}
	u, err := s.store.UserByID(r.Context(), a.session.UserID)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(sha(current), u.AuthHash) != 1 {
		s.audit(r, "rekey_refused")
		if !s.rekeyTries.allow(string(a.tokenHash)) {
			if err := s.store.DeleteSession(r.Context(), a.tokenHash); err != nil {
				return err
			}
			s.setCookie(w, sessionCookie, "", -1)
			s.audit(r, "rekey_logout")
			return fail(http.StatusUnauthorized, "too many wrong passwords; you've been logged out")
		}
		return fail(http.StatusForbidden, "your current password is wrong")
	}
	e, err := body.Enrollment.parse()
	if err != nil {
		return err
	}
	if err := s.store.Rekey(r.Context(), u.ID, a.tokenHash, e); err != nil {
		return err
	}
	if err := s.rotateSession(r.Context(), w, a); err != nil {
		return err
	}
	s.audit(r, "rekey")
	w.WriteHeader(http.StatusNoContent)
	return nil
}
