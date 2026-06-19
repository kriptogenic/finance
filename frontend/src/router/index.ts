import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import AccountsView from '../views/AccountsView.vue'
import TransactionsView from '../views/TransactionsView.vue'
import CategoriesView from '../views/CategoriesView.vue'
import BudgetsView from '../views/BudgetsView.vue'
import ReconciliationView from '../views/ReconciliationView.vue'
import LoginView from '../views/LoginView.vue'
import { isAuthenticated } from '../api/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/', name: 'dashboard', component: DashboardView },
    { path: '/accounts', name: 'accounts', component: AccountsView },
    { path: '/transactions', name: 'transactions', component: TransactionsView },
    { path: '/budgets', name: 'budgets', component: BudgetsView },
    { path: '/categories', name: 'categories', component: CategoriesView },
    { path: '/reconciliation', name: 'reconciliation', component: ReconciliationView },
  ],
})

router.beforeEach((to) => {
  if (!to.meta.public && !isAuthenticated()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && isAuthenticated()) {
    return { name: 'dashboard' }
  }
})

export default router
