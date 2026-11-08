package openapi

import (
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type UUID = openapi_types.UUID
type Email = openapi_types.Email

const (
	ValidationFailed    ErrorCode = ErrorCodeValidationFailed
	BadRequest          ErrorCode = ErrorCodeBadRequest
	Unauthorized        ErrorCode = ErrorCodeUnauthorized
	Forbidden           ErrorCode = ErrorCodeForbidden
	NotFound            ErrorCode = ErrorCodeNotFound
	Conflict            ErrorCode = ErrorCodeConflict
	EmailTaken          ErrorCode = ErrorCodeEmailTaken
	InvalidCredentials  ErrorCode = ErrorCodeInvalidCredentials
	TokenExpired        ErrorCode = ErrorCodeTokenExpired
	OutOfStockError     ErrorCode = ErrorCodeOutOfStock
	PriceChanged        ErrorCode = ErrorCodePriceChanged
	StoreClosed         ErrorCode = ErrorCodeStoreClosed
	IdempotencyConflict ErrorCode = ErrorCodeIdempotencyConflict
	RateLimited         ErrorCode = ErrorCodeRateLimited
	Internal            ErrorCode = ErrorCodeInternal

	VALIDATIONERROR     ErrorCode = ErrorCodeValidationFailed
	BADREQUEST          ErrorCode = ErrorCodeBadRequest
	UNAUTHORIZED        ErrorCode = ErrorCodeUnauthorized
	FORBIDDEN           ErrorCode = ErrorCodeForbidden
	NOTFOUND            ErrorCode = ErrorCodeNotFound
	CONFLICT            ErrorCode = ErrorCodeConflict
	EMAILTAKEN          ErrorCode = ErrorCodeEmailTaken
	INVALIDCREDENTIALS  ErrorCode = ErrorCodeInvalidCredentials
	TOKENEXPIRED        ErrorCode = ErrorCodeTokenExpired
	OUTOFSTOCK          ErrorCode = ErrorCodeOutOfStock
	PRICECHANGED        ErrorCode = ErrorCodePriceChanged
	STORECLOSED         ErrorCode = ErrorCodeStoreClosed
	IDEMPOTENCYCONFLICT ErrorCode = ErrorCodeIdempotencyConflict
	RATELIMITED         ErrorCode = ErrorCodeRateLimited
	INTERNALERROR       ErrorCode = ErrorCodeInternal

	Unavailable       CartLineIssue = CartLineIssueUnavailable
	InsufficientStock CartLineIssue = CartLineIssueInsufficientStock
)
