'use client'

import { useState, useEffect } from 'react'
import Header from '@/components/Header'
import Sidebar from '@/components/Sidebar'

export default function AppShell({ children }: { children: React.ReactNode }) {
  const [darkMode, setDarkMode] = useState(true)
  const [sidebarOpen, setSidebarOpen] = useState(true)

  useEffect(() => {
    const saved = localStorage.getItem('darkMode')
    if (saved !== null) setDarkMode(JSON.parse(saved))
  }, [])

  useEffect(() => {
    localStorage.setItem('darkMode', JSON.stringify(darkMode))
    if (darkMode) {
      document.documentElement.className = 'dark'
    } else {
      document.documentElement.className = 'light'
    }
  }, [darkMode])

  return (
    <>
      <Header
        darkMode={darkMode}
        onToggleDarkMode={() => setDarkMode(!darkMode)}
      />
      <div className="flex pt-14">
        <Sidebar open={sidebarOpen} onToggle={() => setSidebarOpen(!sidebarOpen)} />
        <main className={`flex-1 p-6 transition-all ${sidebarOpen ? 'ml-64' : 'ml-16'}`}>
          {children}
        </main>
      </div>
    </>
  )
}
