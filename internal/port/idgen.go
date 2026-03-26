package port

import "github.com/google/uuid"

type UUIDGenerator struct{}

func (UUIDGenerator) New() string { return uuid.New().String() }
