import { api, call } from './client'
import type { Category, CategoryType, CreateCategoryRequest, UpdateCategoryRequest } from './types'

export const categoriesApi = {
  list: (query: { type?: CategoryType; include_archived?: boolean } = {}): Promise<Category[]> =>
    call(api.GET('/categories', { params: { query } })).then((d) => d.categories),

  get: (id: string): Promise<Category> => call(api.GET('/categories/{id}', { params: { path: { id } } })),

  create: (body: CreateCategoryRequest): Promise<Category> => call(api.POST('/categories', { body })),

  update: (id: string, body: UpdateCategoryRequest): Promise<Category> =>
    call(api.PATCH('/categories/{id}', { params: { path: { id } }, body })),

  remove: (id: string): Promise<unknown> => call(api.DELETE('/categories/{id}', { params: { path: { id } } })),

  suggestIcons: (body: { name: string; type: CategoryType }): Promise<string[]> =>
    call(api.POST('/categories/suggest-icons', { body })).then((d) => d.icons),
}
