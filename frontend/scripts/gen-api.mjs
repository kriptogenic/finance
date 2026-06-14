// Generates src/api/schema.d.ts from ../specs/api.yaml.
//
// This is the openapi-typescript equivalent of the backend's `x-go-type`: the
// `transform` hook maps any schema carrying `x-go-type: money.Money` to our own
// `Money` type, and `inject` adds the import. So the same OpenAPI extension
// drives the Go type (oapi-codegen) and the TS type (here).
import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import openapiTS, { astToString, COMMENT_HEADER } from 'openapi-typescript'
import ts from 'typescript'

const spec = new URL('../../specs/api.yaml', import.meta.url)
const output = fileURLToPath(new URL('../src/api/schema.d.ts', import.meta.url))

const ast = await openapiTS(spec, {
  inject: `import type { Money } from "./money";`,
  transform(schemaObject) {
    if (schemaObject['x-go-type'] === 'money.Money') {
      return ts.factory.createTypeReferenceNode('Money')
    }
    return undefined
  },
})

writeFileSync(output, COMMENT_HEADER + astToString(ast))
