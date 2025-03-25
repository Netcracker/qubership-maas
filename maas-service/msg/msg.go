package msg

import "errors"

var (
	BadRequest = errors.New("input error")          // 400
	AuthError  = errors.New("authentication error") // 403
	NotFound   = errors.New("not found")            // 404
	Conflict   = errors.New("conflict error")       // 409
	Gone       = errors.New("entity gone")          // 410
)

const InvalidClassifierFormat = "invalid classifier '%v': %w"
