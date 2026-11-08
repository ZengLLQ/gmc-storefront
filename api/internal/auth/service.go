package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gmc/api/internal/admin/rbac"
	"gmc/api/internal/audit"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
)

const (
	AccessCookieName    = "gmc_at"
	RefreshCookieName   = "gmc_rt"
	RotationGraceWindow = 10 * time.Second
	AdminIdleTimeout    = 2 * time.Hour
	AdminAbsoluteTTL    = 12 * time.Hour
)

type rotatedPair struct {
	accessJWT  string
	refreshRaw string
	issuedAt   time.Time
}

type lockoutState struct {
	failures    int
	lockedUntil time.Time
}

type Service struct {
	pool    *pgxpool.Pool
	q       *db.Queries
	cfg     *platform.Config
	mu      sync.Mutex
	grace   map[uuid.UUID]rotatedPair
	lockout map[string]lockoutState
}

func NewService(pool *pgxpool.Pool, cfg *platform.Config) *Service {
	return &Service{
		pool:    pool,
		q:       db.New(pool),
		cfg:     cfg,
		grace:   make(map[uuid.UUID]rotatedPair),
		lockout: make(map[string]lockoutState),
	}
}

func (s *Service) SetAuthCookies(w http.ResponseWriter, accessJWT, refreshRaw string, isAdmin bool) {
	sameSite := http.SameSiteLaxMode
	if strings.EqualFold(s.cfg.CookieSameSite, "Strict") {
		sameSite = http.SameSiteStrictMode
	} else if strings.EqualFold(s.cfg.CookieSameSite, "None") {
		sameSite = http.SameSiteNoneMode
	}

	refreshTTL := s.cfg.RefreshTTL
	if isAdmin {
		refreshTTL = AdminIdleTimeout
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    accessJWT,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: sameSite,
		MaxAge:   int(s.cfg.AccessTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    refreshRaw,
		Path:     s.cfg.RefreshCookiePath,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: sameSite,
		MaxAge:   int(refreshTTL.Seconds()),
	})
}

func (s *Service) ClearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     s.cfg.RefreshCookiePath,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		MaxAge:   -1,
	})
}

func (s *Service) IssueSession(ctx context.Context, user db.User, isAdmin bool, userAgent, ip string) (string, string, error) {
	now := time.Now().UTC()
	rawRefresh, tokenHash, err := platform.GenerateOpaqueToken()
	if err != nil {
		return "", "", err
	}
	sessionID := platform.NewUUID()
	familyID := platform.NewUUID()

	expiresAt := now.Add(s.cfg.RefreshTTL)
	var absExpires *time.Time
	if isAdmin {
		expiresAt = now.Add(AdminIdleTimeout)
		abs := now.Add(AdminAbsoluteTTL)
		absExpires = &abs
	}

	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}

	_, err = s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:                sessionID,
		UserID:            user.ID,
		FamilyID:          familyID,
		TokenHash:         tokenHash,
		UserAgent:         uaPtr,
		Column6:           ip,
		ExpiresAt:         expiresAt,
		IsAdminSession:    isAdmin,
		AbsoluteExpiresAt: absExpires,
	})
	if err != nil {
		return "", "", err
	}

	_ = s.q.UpdateUserLastLogin(ctx, user.ID)

	accessJWT, err := platform.SignAccessToken(s.cfg.JWTSecret, user.ID, sessionID, user.Role, isAdmin, now, s.cfg.AccessTTL)
	if err != nil {
		return "", "", err
	}
	return accessJWT, rawRefresh, nil
}

// RefreshSession rotates the refresh token with a 10s grace window for concurrent requests
// and revokes the entire session family if a rotated token is reused outside the grace window.
func (s *Service) RefreshSession(ctx context.Context, refreshRaw, userAgent, ip string) (string, string, bool, error) {
	if strings.TrimSpace(refreshRaw) == "" {
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Sesi tidak ditemukan")
	}
	now := time.Now().UTC()
	hash := platform.HashToken(refreshRaw)

	sess, err := s.q.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Sesi tidak valid")
	}

	if sess.RevokedAt != nil || now.After(sess.ExpiresAt) {
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Sesi telah berakhir")
	}

	if sess.IsAdminSession && sess.AbsoluteExpiresAt != nil && now.After(*sess.AbsoluteExpiresAt) {
		_ = s.q.RevokeSessionByID(ctx, sess.ID)
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Batas waktu sesi admin (12 jam) telah berakhir, silakan login kembali")
	}

	// Handle already-rotated token
	if sess.RotatedAt != nil {
		s.mu.Lock()
		cached, ok := s.grace[sess.ID]
		s.mu.Unlock()
		if ok && now.Sub( *sess.RotatedAt ) <= RotationGraceWindow {
			return cached.accessJWT, cached.refreshRaw, sess.IsAdminSession, nil
		}
		// Reuse outside grace window => suspected token theft => revoke whole family!
		_ = s.q.RevokeSessionFamily(ctx, sess.FamilyID)
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Sesi dicabut karena terdeteksi penggunaan ulang token")
	}

	user, err := s.q.GetUserByID(ctx, sess.UserID)
	if err != nil || !user.IsActive {
		_ = s.q.RevokeSessionFamily(ctx, sess.FamilyID)
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Akun tidak aktif")
	}

	if sess.IsAdminSession && user.Role != "admin" && user.Role != "staff" {
		_ = s.q.RevokeSessionFamily(ctx, sess.FamilyID)
		return "", "", false, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Akses admin telah dicabut")
	}

	newRawRefresh, newHash, err := platform.GenerateOpaqueToken()
	if err != nil {
		return "", "", false, err
	}
	newSessionID := platform.NewUUID()

	expiresAt := now.Add(s.cfg.RefreshTTL)
	if sess.IsAdminSession {
		expiresAt = now.Add(AdminIdleTimeout)
		if sess.AbsoluteExpiresAt != nil && expiresAt.After(*sess.AbsoluteExpiresAt) {
			expiresAt = *sess.AbsoluteExpiresAt
		}
	}

	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", false, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	_, err = qtx.CreateSession(ctx, db.CreateSessionParams{
		ID:                newSessionID,
		UserID:            user.ID,
		FamilyID:          sess.FamilyID,
		TokenHash:         newHash,
		UserAgent:         uaPtr,
		Column6:           ip,
		ExpiresAt:         expiresAt,
		IsAdminSession:    sess.IsAdminSession,
		AbsoluteExpiresAt: sess.AbsoluteExpiresAt,
	})
	if err != nil {
		return "", "", false, err
	}

	if err := qtx.MarkSessionRotated(ctx, db.MarkSessionRotatedParams{
		ID:         sess.ID,
		ReplacedBy: uuid.NullUUID{UUID: newSessionID, Valid: true},
	}); err != nil {
		return "", "", false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", false, err
	}

	accessJWT, err := platform.SignAccessToken(s.cfg.JWTSecret, user.ID, newSessionID, user.Role, sess.IsAdminSession, now, s.cfg.AccessTTL)
	if err != nil {
		return "", "", false, err
	}

	s.mu.Lock()
	s.grace[sess.ID] = rotatedPair{
		accessJWT:  accessJWT,
		refreshRaw: newRawRefresh,
		issuedAt:   now,
	}
	s.mu.Unlock()

	return accessJWT, newRawRefresh, sess.IsAdminSession, nil
}

func (s *Service) CheckAdminLockout(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.lockout[key]
	if !ok {
		return nil
	}
	if time.Now().Before(st.lockedUntil) {
		return platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Akun dikunci sementara selama 15 menit karena terlalu banyak percobaan login yang gagal")
	}
	return nil
}

func (s *Service) RecordAdminLoginFailure(ctx context.Context, key, email, ip string) {
	s.mu.Lock()
	st := s.lockout[key]
	st.failures++
	locked := false
	if st.failures >= 10 {
		st.lockedUntil = time.Now().Add(15 * time.Minute)
		locked = true
	}
	s.lockout[key] = st
	s.mu.Unlock()

	summary := fmt.Sprintf("Percobaan login admin gagal untuk %s", email)
	if locked {
		summary = fmt.Sprintf("Akun admin %s dikunci selama 15 menit setelah 10 kegagalan berturut-turut", email)
	}
	_ = audit.Record(ctx, s.q, audit.Entry{
		ActorEmail: email,
		ActorRole:  "anonymous",
		Action:     "admin.auth.login_failed",
		EntityType: "user",
		Summary:    summary,
		IP:         ip,
	})
}

func (s *Service) ClearAdminLoginFailures(key string) {
	s.mu.Lock()
	delete(s.lockout, key)
	s.mu.Unlock()
}

// VerifyTOTPOrBackup verifies either a 6-digit TOTP code or one of the hashed 8-character backup codes.
func (s *Service) VerifyTOTPOrBackup(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	rec, err := s.q.GetUserTotp(ctx, userID)
	if err != nil || rec.ConfirmedAt == nil {
		return false, nil
	}
	secretBytes, err := platform.DecryptAESGCM(s.cfg.TOTPEncKey, rec.SecretEnc)
	if err != nil {
		return false, err
	}
	code = strings.TrimSpace(code)
	if totp.Validate(code, string(secretBytes)) {
		return true, nil
	}

	// Check backup codes
	codeHash := platform.HashToken(strings.ToUpper(code))
	var remaining [][]byte
	matched := false
	for _, h := range rec.BackupCodesHash {
		if !matched && platform.ConstantTimeHashEqual(h, codeHash) {
			matched = true
			continue
		}
		remaining = append(remaining, h)
	}
	if matched {
		_ = s.q.UpsertUserTotp(ctx, db.UpsertUserTotpParams{
			UserID:          userID,
			SecretEnc:       rec.SecretEnc,
			ConfirmedAt:     rec.ConfirmedAt,
			BackupCodesHash: remaining,
		})
		return true, nil
	}
	return false, nil
}

func GenerateBackupCodes() (plain []string, hashes [][]byte, err error) {
	plain = make([]string, 8)
	hashes = make([][]byte, 8)
	for i := 0; i < 8; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := strings.ToUpper(hex.EncodeToString(b))
		plain[i] = code
		hashes[i] = platform.HashToken(code)
	}
	return plain, hashes, nil
}

// ExtractActor parses the gmc_at cookie and validates the session & user.
func (s *Service) ExtractActor(r *http.Request, requireAdmin bool) (*platform.Actor, *db.User, error) {
	c, err := r.Cookie(AccessCookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return nil, nil, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Silakan masuk terlebih dahulu")
	}

	claims, err := platform.ParseAccessToken(s.cfg.JWTSecret, c.Value)
	if err != nil {
		if err == platform.ErrTokenExpired {
			return nil, nil, platform.NewAppError(http.StatusUnauthorized, openapi.TokenExpired, "Sesi akses telah kedaluwarsa")
		}
		return nil, nil, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Token akses tidak valid")
	}

	if requireAdmin && !claims.IsAdmin {
		return nil, nil, platform.NewAppError(http.StatusForbidden, openapi.Forbidden, "Sesi ini bukan sesi admin")
	}

	// Re-read user & session from DB so demotion/deactivation/revocation is immediate
	sess, err := s.q.GetSessionByID(r.Context(), claims.SessionID)
	if err != nil || sess.RevokedAt != nil {
		return nil, nil, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Sesi telah berakhir")
	}

	user, err := s.q.GetUserByID(r.Context(), claims.UserID)
	if err != nil || !user.IsActive {
		return nil, nil, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Akun tidak aktif")
	}

	if requireAdmin && user.Role != "admin" && user.Role != "staff" {
		return nil, nil, platform.NewAppError(http.StatusForbidden, openapi.Forbidden, "Anda tidak memiliki akses ke panel admin")
	}

	actor := &platform.Actor{
		UserID:    user.ID.String(),
		Email:     user.Email,
		Role:      user.Role,
		SessionID: sess.ID.String(),
		IsAdmin:   claims.IsAdmin,
	}
	return actor, &user, nil
}

func (s *Service) RequirePermission(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (*platform.Actor, *db.User, bool) {
	actor, user, err := s.ExtractActor(r, true)
	if err != nil {
		platform.WriteError(w, r, err)
		return nil, nil, false
	}
	if !rbac.HasPermission(user.Role, perm) {
		_ = audit.Record(r.Context(), s.q, audit.Entry{
			ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
			ActorEmail: user.Email,
			ActorRole:  user.Role,
			Action:     "admin.permission_denied",
			EntityType: "route",
			EntityID:   r.URL.Path,
			Summary:    fmt.Sprintf("Akses ditolak ke %s %s (membutuhkan %s)", r.Method, r.URL.Path, perm),
		})
		platform.WriteError(w, r, platform.NewAppError(http.StatusForbidden, openapi.Forbidden, "Anda tidak memiliki izin untuk melakukan tindakan ini"))
		return nil, nil, false
	}
	return actor, user, true
}
