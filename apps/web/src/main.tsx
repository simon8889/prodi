import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import App from './App'

import './styles/tokens.css'
import './styles/global.css'
import './styles/desktop.css'
import './styles/controls.css'
import './styles/window.css'
import './styles/todo.css'
import './styles/pomodoro.css'
import './styles/stats.css'
import './styles/boot.css'

const container = document.getElementById('root')

if (!container) {
  throw new Error('Root container #root is missing from index.html')
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
