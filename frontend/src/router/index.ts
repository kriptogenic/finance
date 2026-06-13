import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import AccountsView from '../views/AccountsView.vue'
import TransactionsView from '../views/TransactionsView.vue'
import CategoriesView from '../views/CategoriesView.vue'
import BudgetsView from '../views/BudgetsView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardView },
    { path: '/accounts', name: 'accounts', component: AccountsView },
    { path: '/transactions', name: 'transactions', component: TransactionsView },
    { path: '/budgets', name: 'budgets', component: BudgetsView },
    { path: '/categories', name: 'categories', component: CategoriesView },
  ],
})

export default router
