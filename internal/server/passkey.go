package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/DimwitLabs/tuck/internal/store"
)

const (
	ceremonyTTL = 5 * time.Minute
	maxPasskeys = 10
	labelMax    = 40
)

// Chrome reaches for the phone-and-QR flow unless the hint keeps it on the sensor this browser already has.
var thisDevice = []protocol.PublicKeyCredentialHints{protocol.PublicKeyCredentialHintClientDevice}

// A passkey only opens the door; the vault still wants the password and the three answers.
func newWebAuthn(origin, gateWord string) (*webauthn.WebAuthn, error) {
	if origin == "" {
		return nil, nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || net.ParseIP(u.Hostname()) != nil || (u.Scheme != "https" && u.Hostname() != "localhost") {
		return nil, errors.New("TUCK_ORIGIN must be the https address tuck answers on by name, such as https://tuck.example.com")
	}
	name := gateWord
	if name == "" {
		name = u.Hostname()
	}
	return webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: name,
		RPOrigins:     []string{strings.TrimSuffix(u.Scheme+"://"+u.Host, "/")},
	})
}

// Cut by runes: half a rune is not valid utf-8, and postgres refuses to store it.
func deviceLabel(raw string) string {
	label := strings.TrimSpace(raw)
	if label == "" {
		return "this device"
	}
	if runes := []rune(label); len(runes) > labelMax {
		return string(runes[:labelMax])
	}
	return label
}

type passkeyUser struct {
	id    string
	name  string
	creds []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte                         { return []byte(u.id) }
func (u *passkeyUser) WebAuthnName() string                       { return u.name }
func (u *passkeyUser) WebAuthnDisplayName() string                { return u.name }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func credentialOf(p store.Passkey) webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, 0, len(p.Transports))
	for _, t := range p.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(t))
	}
	return webauthn.Credential{
		ID:        p.CredentialID,
		PublicKey: p.PublicKey,
		Transport: transports,
		Flags:     webauthn.CredentialFlags{BackupEligible: p.BackedUp, BackupState: p.BackedUp},
		Authenticator: webauthn.Authenticator{
			AAGUID:    p.AAGUID,
			SignCount: p.SignCount,
		},
	}
}

func (s *Server) passkeyUser(r *http.Request, userID string) (*passkeyUser, error) {
	u, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	keys, err := s.store.Passkeys(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	user := &passkeyUser{id: userID, name: u.Username}
	for _, k := range keys {
		user.creds = append(user.creds, credentialOf(k))
	}
	return user, nil
}

// Android's credential manager refuses a whole registration when something in
// the exclusion list claims a transport it would have to leave the device for.
func onThisDevice(in []protocol.AuthenticatorTransport) []protocol.AuthenticatorTransport {
	out := make([]protocol.AuthenticatorTransport, 0, len(in))
	for _, t := range in {
		if t == protocol.Internal {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) passkeysOn() error {
	if s.auth == nil {
		return fail(http.StatusNotFound, "passkeys are off; set TUCK_ORIGIN to turn them on")
	}
	return nil
}

func (s *Server) beginPasskey(w http.ResponseWriter, r *http.Request) error {
	if err := s.passkeysOn(); err != nil {
		return err
	}
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	user, err := s.passkeyUser(r, a.session.UserID)
	if err != nil {
		return err
	}
	if len(user.creds) >= maxPasskeys {
		return fail(http.StatusConflict, "that is as many devices as one vault can hold")
	}
	exclude := make([]protocol.CredentialDescriptor, 0, len(user.creds))
	for _, c := range user.creds {
		d := c.Descriptor()
		d.Transport = onThisDevice(d.Transport)
		exclude = append(exclude, d)
	}
	creation, session, err := s.auth.BeginRegistration(user,
		webauthn.WithExclusions(exclude),
		webauthn.WithPublicKeyCredentialHints(thisDevice),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return err
	}
	// The hint asks for this device without binding it. go-webauthn turns the
	// hint into a platform attachment, which sends android straight past
	// whichever passkey provider the phone actually uses.
	creation.Response.AuthenticatorSelection.AuthenticatorAttachment = ""
	if err := s.keepCeremony(w, registerCookie, session); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, creation.Response)
	return nil
}

func (s *Server) finishPasskey(w http.ResponseWriter, r *http.Request) error {
	if err := s.passkeysOn(); err != nil {
		return err
	}
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	session, err := s.takeCeremony(w, r, registerCookie)
	if err != nil {
		return err
	}
	user, err := s.passkeyUser(r, a.session.UserID)
	if err != nil {
		return err
	}
	label := deviceLabel(r.URL.Query().Get("label"))
	cred, err := s.auth.FinishRegistration(user, *session, r)
	if err != nil {
		slog.Warn("passkey enrolment refused", "ip", s.clientIP(r), "err", err)
		return fail(http.StatusBadRequest, "that device could not be enrolled")
	}
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	err = s.store.AddPasskey(r.Context(), store.Passkey{
		CredentialID: cred.ID,
		UserID:       a.session.UserID,
		PublicKey:    cred.PublicKey,
		AAGUID:       cred.Authenticator.AAGUID,
		SignCount:    cred.Authenticator.SignCount,
		BackedUp:     cred.Flags.BackupState,
		Transports:   transports,
		Label:        label,
	})
	if err != nil {
		return err
	}
	s.audit(r, "passkey_added")
	return s.listPasskeys(w, r)
}

func (s *Server) listPasskeys(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	keys, err := s.store.Passkeys(r.Context(), a.session.UserID)
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"id":       base64.RawURLEncoding.EncodeToString(k.CredentialID),
			"label":    k.Label,
			"added":    k.CreatedAt,
			"lastUsed": k.LastUsedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"passkeys": out,
		"enabled":  s.auth != nil,
		"userId":   base64.RawURLEncoding.EncodeToString([]byte(a.session.UserID)),
	})
	return nil
}

func (s *Server) deletePasskey(w http.ResponseWriter, r *http.Request) error {
	a, err := s.unlocked(r)
	if err != nil {
		return err
	}
	id, err := base64.RawURLEncoding.DecodeString(r.PathValue("id"))
	if err != nil {
		return fail(http.StatusBadRequest, "that is not a device id")
	}
	if err := s.store.DeletePasskey(r.Context(), a.session.UserID, id); errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusNotFound, "that device is not enrolled")
	} else if err != nil {
		return err
	}
	s.audit(r, "passkey_removed")
	return s.listPasskeys(w, r)
}

func (s *Server) gatePasskeyStart(w http.ResponseWriter, r *http.Request) error {
	if err := s.passkeysOn(); err != nil {
		return err
	}
	if !s.gateByIP.allow(limiterKey(s.clientIP(r))) {
		return errGateClosed
	}
	assertion, session, err := s.auth.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
		webauthn.WithAssertionPublicKeyCredentialHints(thisDevice),
	)
	if err != nil {
		return err
	}
	if err := s.keepCeremony(w, gatePasskeyCookie, session); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, assertion.Response)
	return nil
}

func (s *Server) gatePasskeyFinish(w http.ResponseWriter, r *http.Request) error {
	if err := s.passkeysOn(); err != nil {
		return err
	}
	if !s.gateByIP.allow(limiterKey(s.clientIP(r))) {
		return errGateClosed
	}
	session, err := s.takeCeremony(w, r, gatePasskeyCookie)
	if err != nil {
		return errGateClosed
	}
	var used *store.Passkey
	cred, err := s.auth.FinishDiscoverableLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		p, err := s.store.PasskeyByID(r.Context(), rawID)
		if err != nil {
			return nil, err
		}
		if string(userHandle) != p.UserID {
			return nil, errors.New("that passkey belongs to another account")
		}
		used = p
		return s.passkeyUser(r, p.UserID)
	}, *session, r)
	if err != nil || used == nil {
		return errGateClosed
	}
	if err := s.store.PasskeyUsed(r.Context(), used.CredentialID, cred.Authenticator.SignCount); err != nil {
		return err
	}
	s.setCookie(w, gateCookie, s.signed(s.gatePurpose(), gateTTL), int(gateTTL.Seconds()))
	s.audit(r, "gate_opened_passkey")
	writeJSON(w, http.StatusOK, map[string]bool{"open": true})
	return nil
}

func (s *Server) keepCeremony(w http.ResponseWriter, name string, session *webauthn.SessionData) error {
	blob, err := json.Marshal(session)
	if err != nil {
		return err
	}
	s.setCookie(w, name, s.sealed(name, blob, ceremonyTTL), int(ceremonyTTL.Seconds()))
	return nil
}

func (s *Server) takeCeremony(w http.ResponseWriter, r *http.Request, name string) (*webauthn.SessionData, error) {
	blob, ok := s.unsealed(name, s.cookie(r, name))
	s.setCookie(w, name, "", -1)
	if !ok {
		return nil, fail(http.StatusBadRequest, "that took too long; try again")
	}
	session := &webauthn.SessionData{}
	if err := json.Unmarshal(blob, session); err != nil {
		return nil, fail(http.StatusBadRequest, "that took too long; try again")
	}
	return session, nil
}
