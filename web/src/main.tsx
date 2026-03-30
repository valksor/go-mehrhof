import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ScreenReaderAnnouncer } from './components/ui/ScreenReaderAnnouncer'
import { SkipLink } from './components/ui/SkipLink'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ScreenReaderAnnouncer>
      <SkipLink />
      <App />
    </ScreenReaderAnnouncer>
  </StrictMode>,
)
