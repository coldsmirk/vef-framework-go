package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type submissionRequest struct {
	ApplicantID string
	FlowCode    string
	InstanceID  string
	BusinessRef *string
	FormData    map[string]any
}

type submissionKey struct {
	orm.BaseModel `bun:"table:apv_submission_key,alias:ask"`

	TenantID    string  `bun:"tenant_id,pk"`
	Key         string  `bun:"idempotency_key,pk"`
	Operation   string  `bun:"operation"`
	Fingerprint string  `bun:"fingerprint"`
	InstanceID  *string `bun:"instance_id"`
}

func newSubmissionKey(key, op string, req submissionRequest) (*submissionKey, error) {
	if !utf8.ValidString(key) || strings.ContainsRune(key, 0) || utf8.RuneCountInString(key) > 128 {
		return nil, errInvalidIdempotencyKey
	}

	if req.FormData == nil {
		req.FormData = map[string]any{}
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode submission fingerprint: %w", err)
	}

	// Canonicalize object order and strings, including embedded RawMessages.
	// RFC 8785 rounds numbers to binary64; preserve them here and normalize
	// their exact decimal values while hashing.
	canonical := jsontext.Value(raw)
	if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false), jsontext.CanonicalizeRawFloats(false)); err != nil {
		return nil, fmt.Errorf("canonicalize submission fingerprint: %w", err)
	}

	hash := sha256.New()
	decoder := jsontext.NewDecoder(bytes.NewReader(canonical))

	var offset int64
	for decoder.InputOffset() < int64(len(canonical)) {
		token, err := decoder.ReadToken()
		if err != nil {
			return nil, fmt.Errorf("read submission fingerprint: %w", err)
		}

		if token.Kind() == jsontext.KindNumber {
			literal := token.String()
			end := decoder.InputOffset()
			_, _ = hash.Write(canonical[offset : end-int64(len(literal))])
			_, _ = hash.Write([]byte(exactSubmissionNumber(literal)))
			offset = end
		}
	}

	_, _ = hash.Write(canonical[offset:])

	return &submissionKey{Key: key, Operation: op, Fingerprint: hex.EncodeToString(hash.Sum(nil))}, nil
}

// Normalize a JSON decimal to coefficient × 10^exponent. Only the exponent
// is added, so even 1e1000000000 never expands into a billion-digit integer.
func exactSubmissionNumber(number string) string {
	literal, negative := strings.CutPrefix(number, "-")
	mantissa, exponent, _ := strings.Cut(strings.ToLower(literal), "e")
	whole, fraction, _ := strings.Cut(mantissa, ".")

	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0"
	}

	coefficient := strings.TrimRight(digits, "0")

	scale := big.NewInt(int64(len(digits) - len(coefficient) - len(fraction)))
	if exponent != "" {
		value, _ := new(big.Int).SetString(exponent, 10) // Validated by the JSON decoder.
		scale.Add(scale, value)
	}

	if negative {
		coefficient = "-" + coefficient
	}

	return coefficient + "e" + scale.String()
}

// reserve claims a key in the authoritative tenant before the handler runs.
func (s *submissionKey) reserve(ctx context.Context, db orm.DB) (bool, error) {
	res, err := db.NewInsert().Model(s).
		Select("tenant_id", "idempotency_key", "operation", "fingerprint").
		OnConflict(func(cb orm.ConflictBuilder) { cb.Columns("tenant_id", "idempotency_key").DoNothing() }).
		ReturningNone().Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("reserve submission key: %w", err)
	}

	count, err := res.RowsAffected()

	return count == 1, err
}

// SQLite's deferred transaction must begin with a write. Derive the tenant
// from the target in the reservation itself, before reading the instance.
func (s *submissionKey) reserveSQLiteResubmission(ctx context.Context, db orm.DB, tenant *string, target string) (bool, error) {
	query := "INSERT INTO apv_submission_key (tenant_id, idempotency_key, operation, fingerprint) " +
		"SELECT tenant_id, ?, ?, ? FROM apv_instance WHERE id = ?"

	args := []any{s.Key, s.Operation, s.Fingerprint, target}
	if tenant != nil {
		query += " AND tenant_id = ?"

		args = append(args, *tenant)
	}

	res, err := db.NewRaw(query+" ON CONFLICT (tenant_id, idempotency_key) DO NOTHING", args...).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("reserve resubmission key: %w", err)
	}

	count, err := res.RowsAffected()

	return count == 1, err
}

func (s *submissionKey) replay(ctx context.Context, db orm.DB) (string, error) {
	saved := submissionKey{TenantID: s.TenantID, Key: s.Key}
	// The competing writer may have committed after the caller's MySQL
	// repeatable-read snapshot. Both the key and its instance need current reads.
	if err := db.NewSelect().Model(&saved).WherePK().ForShare().Scan(ctx); err != nil {
		return "", fmt.Errorf("read submission key: %w", err)
	}

	if saved.InstanceID == nil || saved.Fingerprint != s.Fingerprint || saved.Operation != s.Operation {
		return "", approval.ErrIdempotencyConflict
	}

	return *saved.InstanceID, nil
}

// link supplies the foreign key once the instance exists. No open reservation
// survives a rollback, and deleting an instance removes all its submissions.
func (s *submissionKey) link(ctx context.Context, db orm.DB, id string) error {
	s.InstanceID = &id
	_, err := db.NewUpdate().Model(s).Select("instance_id").WherePK().Exec(ctx)

	return err
}
