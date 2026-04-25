package helpers

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func ToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func FromPgUUID(id pgtype.UUID) uuid.UUID {
	return id.Bytes
}

func ToPgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func ToPgTimestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

func PgToTime(pgTime pgtype.Timestamptz) time.Time {

	return pgTime.Time
}

type UUIDField struct {
	Value string
	Name  string
	Dest  *uuid.UUID
}

func ParseUUIDs(fields ...UUIDField) error {
	for _, f := range fields {
		id, err := uuid.Parse(f.Value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", f.Name, err)
		}
		*f.Dest = id
	}
	return nil
}

func ParseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
