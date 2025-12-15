import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import Login from '../views/Login.vue'
import Overview from '../views/Overview.vue'
import Docker from '../views/Docker.vue'
import Images from '../views/Images.vue'
import Volumes from '../views/Volumes.vue'
import Networks from '../views/Networks.vue'
import AppStore from '../views/AppStore.vue'
import AppDeploy from '../views/AppDeploy.vue'
import Navigation from '../views/Navigation.vue'
import Projects from '../views/Projects.vue'
import ProjectDetail from '../views/ProjectDetail.vue'
import DockerDetail from '../views/DockerDetail.vue'
import Settings from '../views/Settings.vue'
import Ports from '../views/Ports.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: Login
    },
    {
      path: '/',
      component: MainLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          redirect: '/overview'
        },
        {
          path: 'overview',
          component: Overview
        },
        {
          path: 'containers',
          component: Docker
        },
        {
          path: 'containers/:name',
          component: DockerDetail
        },
        {
          path: 'app-store',
          component: AppStore
        },
        {
          path: 'appstore/deploy/:projectId',
          component: AppDeploy
        },
        {
          path: 'images',
          component: Images
        },
        {
          path: 'volumes',
          component: Volumes
        },
        {
          path: 'networks',
          component: Networks
        },
        {
          path: 'ports',
          component: Ports
        },
        {
          path: 'navigation',
          component: Navigation
        },
        {
          path: 'projects',
          component: Projects
        },
        {
          path: 'projects/:name',
          component: ProjectDetail
        },
        {
          path: 'settings',
          component: Settings
        }
      ]
    }
  ]
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next({
      path: '/login',
      query: { redirect: to.fullPath }
    })
  } else {
    next()
  }
})

export default router
