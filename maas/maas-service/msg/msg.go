package msg

import "errors"

var (
	//nolint:staticcheck // ST1012: legacy exported names used across the project
	BadRequest = errors.New("input error") // 400
	//nolint:staticcheck // ST1012: legacy exported names used across the project
	AuthError = errors.New("authentication error") // 403
	//nolint:staticcheck // ST1012: legacy exported names used across the project
	NotFound = errors.New("not found") // 404
	//nolint:staticcheck // ST1012: legacy exported names used across the project
	Conflict = errors.New("conflict error") // 409
	//nolint:staticcheck // ST1012: legacy exported names used across the project
	Gone = errors.New("entity gone") // 410
)

const InvalidClassifierFormat = "invalid classifier '%v': %w"
