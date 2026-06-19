import { api, call } from './client'
import type { CategoryRuleBlock } from './types'

export const ruleBlocksApi = {
  list: (): Promise<CategoryRuleBlock[]> =>
    call(api.GET('/category-rule-blocks')).then((d) => d.blocks),

  create: (merchant: string): Promise<CategoryRuleBlock> =>
    call(api.POST('/category-rule-blocks', { body: { merchant } })),
}
