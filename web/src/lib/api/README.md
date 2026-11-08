# Storefront API Client

This directory contains the client-side data layer connecting the Next.js storefront to the Go backend.

- `schema.d.ts`: Strictly typed TypeScript interfaces generated from the OpenAPI 3.1 specification (`api/openapi.yaml`).
- `client.ts`: Typed fetch wrapper configured to talk to `/api/v1` (the Next.js API proxy) with error handling, authentication bearer attachment, and timeouts.
- `mappers.ts`: Helper mappers converting raw API response structures into display DTOs (`ProductCardDTO`, `ProductDetailDTO`, etc.) with fallback mock data when developing offline.
- `errors.ts`: Standardized error types handling RFC-7807 problem details and validation errors.
