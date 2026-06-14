// Custom TS type for the OpenAPI `Money` schema — the openapi-typescript analog
// of the backend's `x-go-type: money.Money`. The generator (scripts/gen-api.mjs)
// maps any schema carrying `x-go-type: money.Money` to this type and imports it.
//
// The wire shape is integer minor units + ISO-4217 code; all behaviour (format,
// compare, arithmetic, unit conversion) goes through dinero.js in lib/format.ts.
export interface Money {
  /** amount in integer minor units (e.g. cents); never floating point */
  amount: number
  /** ISO-4217 currency code */
  currency: string
}
