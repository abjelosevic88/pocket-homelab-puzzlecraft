import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/puzzlecraft.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Today, { DailyPlay } from './pages/Today'
import Puzzles from './pages/Puzzles'
import NewPuzzle from './pages/NewPuzzle'
import PuzzlePage from './pages/PuzzlePage'
import Books, { BookPage } from './pages/Books'
import Adventures from './pages/Adventures'
import AdventurePage from './pages/AdventurePage'
import Words from './pages/Words'
import Stats from './pages/Stats'
import { PrintAdventure, PrintBook, PrintPuzzle } from './pages/Print'
import { PublicAdventure, PublicBook, PublicPuzzle } from './pages/Public'
import Settings from './pages/Settings'
import { DataProvider } from './lib/data'

function Gate({ children }: { children: React.ReactNode }) {
  const { status, user } = useApp()
  const loc = useLocation()
  if (!status) return <div className="boot"><img src="/favicon.svg" alt="" /><Loading /></div>
  if (status.needs_setup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login mode="login" />} />
      <Route path="/setup" element={<Login mode="setup" />} />
      <Route path="/p/:token" element={<ErrorBoundary where="shared"><PublicPuzzle /></ErrorBoundary>} />
      <Route path="/b/:token" element={<ErrorBoundary where="shared"><PublicBook /></ErrorBoundary>} />
      <Route path="/a/:token" element={<ErrorBoundary where="shared"><PublicAdventure /></ErrorBoundary>} />
      <Route path="/puzzles/:id/print" element={<Gate><ErrorBoundary where="print"><PrintPuzzle /></ErrorBoundary></Gate>} />
      <Route path="/books/:id/print" element={<Gate><ErrorBoundary where="print"><PrintBook /></ErrorBoundary></Gate>} />
      <Route path="/adventures/:id/print" element={<Gate><ErrorBoundary where="print"><PrintAdventure /></ErrorBoundary></Gate>} />
      <Route path="/settings/:tab?" element={<Gate><ErrorBoundary where="settings"><DataProvider><Settings /></DataProvider></ErrorBoundary></Gate>} />
      <Route element={<Gate><ErrorBoundary where="puzzlecraft"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Today />} />
        <Route path="/today" element={<Today />} />
        <Route path="/today/:date/:kind" element={<DailyPlay />} />
        <Route path="/puzzles" element={<Puzzles />} />
        <Route path="/new" element={<NewPuzzle />} />
        <Route path="/puzzles/:id" element={<PuzzlePage />} />
        <Route path="/books" element={<Books />} />
        <Route path="/books/:id" element={<BookPage />} />
        <Route path="/adventures" element={<Adventures />} />
        <Route path="/adventures/:id" element={<AdventurePage />} />
        <Route path="/words" element={<Words />} />
        <Route path="/stats" element={<Stats />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ErrorBoundary where="app"><ToastProvider><AppProvider><App /></AppProvider></ToastProvider></ErrorBoundary>
    </BrowserRouter>
  </React.StrictMode>,
)
