package audit

import (
	"context"
	"encoding/json"
	"strings"

	"gmc/api/internal/gen/db"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
)

var sensitiveKeys = []string{
	"password", "password_hash", "passwordhash",
	"secret", "secret_enc", "totp", "backup_codes", "backupcodes",
	"token", "token_hash", "view_token", "refresh_token",
}

func RedactMap(in any) []byte {
	if in == nil {
		return nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	redactRecursive(m)
	out, _ := json.Marshal(m)
	return out
}

func redactRecursive(m map[string]any) {
	for k, v := range m {
		lower := strings.ToLower(k)
		isSensitive := false
		for _, sk := range sensitiveKeys {
			if strings.Contains(lower, sk) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			m[k] = "[REDACTED]"
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			redactRecursive(sub)
		}
	}
}

type Entry struct {
	ActorID    uuid.NullUUID
	ActorEmail string
	ActorRole  string
	Action     string
	EntityType string
	EntityID   string
	Summary    string
	Before     any
	After      any
	IP         string
	RequestID  string
}

func Record(ctx context.Context, q *db.Queries, e Entry) error {
	actor := platform.ActorFromContext(ctx)
	if actor != nil {
		if e.ActorEmail == "" {
			e.ActorEmail = actor.Email
		}
		if e.ActorRole == "" {
			e.ActorRole = actor.Role
		}
		if !e.ActorID.Valid && actor.UserID != "" {
			if uid, err := uuid.Parse(actor.UserID); err == nil {
				e.ActorID = uuid.NullUUID{UUID: uid, Valid: true}
			}
		}
	}
	if e.ActorEmail == "" {
		e.ActorEmail = "system"
	}
	if e.ActorRole == "" {
		e.ActorRole = "system"
	}
	if e.IP == "" {
		e.IP = platform.ClientIPFromContext(ctx)
	}
	if e.RequestID == "" {
		e.RequestID = platform.RequestIDFromContext(ctx)
	}

	var entityIDPtr *string
	if e.EntityID != "" {
		entityIDPtr = &e.EntityID
	}
	var reqIDPtr *string
	if e.RequestID != "" {
		reqIDPtr = &e.RequestID
	}
	_, err := q.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ID:         platform.NewUUID(),
		ActorID:    e.ActorID,
		ActorEmail: e.ActorEmail,
		ActorRole:  e.ActorRole,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   entityIDPtr,
		Summary:    e.Summary,
		Before:     RedactMap(e.Before),
		After:      RedactMap(e.After),
		Column11:   e.IP,
		RequestID:  reqIDPtr,
	})
	return err
}
